package mirabase

import (
	"fmt"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"mirabase/cmd"
	"mirabase/core"
	"mirabase/tools/hook"
	"mirabase/tools/list"
	"mirabase/tools/osutils"
	"mirabase/tools/routine"
	"github.com/spf13/cobra"

	_ "mirabase/migrations"
)

var _ core.App = (*MiraBase)(nil)

// Version of MiraBase
var Version = "(untracked)"

// MiraBase defines a MiraBase app launcher.
//
// It implements [core.App] via embedding and all of the app interface methods
// could be accessed directly through the instance (eg. MiraBase.DataDir()).
type MiraBase struct {
	core.App

	devFlag           bool
	dataDirFlag       string
	encryptionEnvFlag string
	queryTimeout      int
	hideStartBanner   bool

	// RootCmd is the main console command
	RootCmd *cobra.Command

	// set when the default data directory discovery is ambiguous and
	// the app bootstrap must be refused (see resolveDataDirDiscovery)
	dirDiscoveryErr error
}

// Config is the MiraBase initialization config struct.
type Config struct {
	// hide the default console server info on app startup
	HideStartBanner bool

	// optional default values for the console flags
	DefaultDev           bool
	DefaultDataDir       string // if not set, it will fallback to "./mb_data"
	DefaultEncryptionEnv string
	DefaultQueryTimeout  time.Duration // default to core.DefaultQueryTimeout (in seconds)

	// optional DB configurations
	DataMaxOpenConns int                // default to core.DefaultDataMaxOpenConns
	DataMaxIdleConns int                // default to core.DefaultDataMaxIdleConns
	AuxMaxOpenConns  int                // default to core.DefaultAuxMaxOpenConns
	AuxMaxIdleConns  int                // default to core.DefaultAuxMaxIdleConns
	DBConnect        core.DBConnectFunc // default to core.dbConnect
}

// New creates a new MiraBase instance with the default configuration.
// Use [NewWithConfig] if you want to provide a custom configuration.
//
// Note that the application will not be initialized/bootstrapped yet,
// aka. DB connections, migrations, app settings, etc. will not be accessible.
// Everything will be initialized when [MiraBase.Start] is executed.
// If you want to initialize the application before calling [MiraBase.Start],
// then you'll have to manually call [MiraBase.Bootstrap].
func New() *MiraBase {
	_, isUsingGoRun := inspectRuntime()

	return NewWithConfig(Config{
		DefaultDev: isUsingGoRun,
	})
}

// NewWithConfig creates a new MiraBase instance with the provided config.
//
// Note that the application will not be initialized/bootstrapped yet,
// aka. DB connections, migrations, app settings, etc. will not be accessible.
// Everything will be initialized when [MiraBase.Start] is executed.
// If you want to initialize the application before calling [MiraBase.Start],
// then you'll have to manually call [MiraBase.Bootstrap].
func NewWithConfig(config Config) *MiraBase {
	// initialize a default data directory based on the executable baseDir
	baseDir, _ := inspectRuntime()
	autoDataDir := config.DefaultDataDir == ""
	if autoDataDir {
		config.DefaultDataDir = filepath.Join(baseDir, "mb_data")
	}

	if config.DefaultQueryTimeout == 0 {
		config.DefaultQueryTimeout = core.DefaultQueryTimeout
	}

	executableName := filepath.Base(os.Args[0])

	pb := &MiraBase{
		RootCmd: &cobra.Command{
			Use:     executableName,
			Short:   executableName + " CLI",
			Version: Version,
			FParseErrWhitelist: cobra.FParseErrWhitelist{
				UnknownFlags: true,
			},
			// no need to provide the default cobra completion command
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: true,
			},
		},
		devFlag:           config.DefaultDev,
		dataDirFlag:       config.DefaultDataDir,
		encryptionEnvFlag: config.DefaultEncryptionEnv,
		hideStartBanner:   config.HideStartBanner,
	}

	// don't write command errors to the stderr because the error is
	// propagated back to the app.Start() and could result in duplication
	pb.RootCmd.SetErr(&nopWrite{})

	// parse base flags
	// (errors are ignored, since the full flags parsing happens on Execute())
	pb.eagerParseFlags(&config)

	// enforce the fresh mb_data / legacy pb_data discovery contract
	// (only when the data directory was not explicitly selected)
	if autoDataDir {
		pb.resolveDataDirDiscovery(baseDir)
	}

	// initialize the app instance
	pb.App = core.NewBaseApp(core.BaseAppConfig{
		IsDev:            pb.devFlag,
		DataDir:          pb.dataDirFlag,
		EncryptionEnv:    pb.encryptionEnvFlag,
		QueryTimeout:     time.Duration(pb.queryTimeout) * time.Second,
		DataMaxOpenConns: config.DataMaxOpenConns,
		DataMaxIdleConns: config.DataMaxIdleConns,
		AuxMaxOpenConns:  config.AuxMaxOpenConns,
		AuxMaxIdleConns:  config.AuxMaxIdleConns,
		DBConnect:        config.DBConnect,
	})

	// hide the default help command (allow only `--help` flag)
	pb.RootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	pb.OnBootstrap().Bind(&hook.Handler[*core.BootstrapEvent]{
		Id: ModerncDepsCheckHookId,
		Func: func(be *core.BootstrapEvent) error {
			if err := be.Next(); err != nil {
				return err
			}

			// run separately to avoid blocking
			app := be.App
			routine.FireAndForget(func() {
				checkModerncDeps(app)
			})

			return nil
		},
	})

	return pb
}

// Start starts the application, aka. registers the default system
// commands (serve, superuser, version) and executes pb.RootCmd.
func (pb *MiraBase) Start() error {
	// register system commands
	pb.RootCmd.AddCommand(cmd.NewSuperuserCommand(pb))
	pb.RootCmd.AddCommand(cmd.NewServeCommand(pb, !pb.hideStartBanner))

	return pb.Execute()
}

// Execute initializes the application (if not already) and executes
// the pb.RootCmd with graceful shutdown support.
//
// This method differs from pb.Start() by not registering the default
// system commands!
func (pb *MiraBase) Execute() error {
	if !pb.skipBootstrap() {
		if err := pb.Bootstrap(); err != nil {
			return err
		}
	}

	execCh := make(chan error, 1)
	sigCh := make(chan os.Signal, 1)

	// listen for interrupt signal to gracefully shutdown the application
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// execute the root command
	go func() {
		execCh <- routine.SafeWrap(pb.RootCmd.Execute)()
	}()

	// wait for either an OS signal or the command to complete
	var execErr error
	select {
	case <-sigCh:
	case execErr = <-execCh:
	}

	signal.Stop(sigCh)

	// trigger cleanups
	//
	// @todo consider skipping and just call the finalizer in case OnTerminate was already invoked manually?
	event := new(core.TerminateEvent)
	event.App = pb
	return pb.OnTerminate().Trigger(event, func(e *core.TerminateEvent) error {
		return errors.Join(e.App.ClearBootstrap(), execErr)
	})
}

// Bootstrap initializes the application, refusing the startup when the
// default data directory discovery is ambiguous (see resolveDataDirDiscovery).
//
// This method is otherwise equivalent to the embedded [core.App.Bootstrap].
func (pb *MiraBase) Bootstrap() error {
	if pb.dirDiscoveryErr != nil {
		return pb.dirDiscoveryErr
	}

	return pb.App.Bootstrap()
}

// resolveDataDirDiscovery enforces the fresh mb_data / legacy pb_data
// discovery contract when the data directory was not explicitly selected:
//
//   - both mb_data and pb_data exist in the base directory -> refuse the
//     ambiguous startup until --dir explicitly selects one
//   - only the legacy pb_data exists -> warn and continue with the fresh
//     mb_data directory (a new empty one will be created next to it)
func (pb *MiraBase) resolveDataDirDiscovery(baseDir string) {
	if dirFlag := pb.RootCmd.PersistentFlags().Lookup("dir"); dirFlag != nil && dirFlag.Changed {
		return // the operator explicitly selected the data directory
	}

	freshDir := pb.dataDirFlag // the default ./mb_data location
	legacyDir := filepath.Join(baseDir, "pb_data")

	if freshDir == legacyDir || !pathExists(legacyDir) {
		return // no legacy discovery conflict
	}

	if pathExists(freshDir) {
		pb.dirDiscoveryErr = fmt.Errorf(
			"ambiguous data directory: both %q and %q exist in %q – use --dir to explicitly select the directory to operate on",
			filepath.Base(freshDir), filepath.Base(legacyDir), baseDir,
		)
		return
	}

	fmt.Fprintf(os.Stderr,
		"WARNING: found a legacy %q directory in %q.\n"+
			"         MiraBase will continue with the fresh %q directory (created empty on start).\n"+
			"         Run with --dir %q to operate on the legacy directory instead.\n",
		filepath.Base(legacyDir), baseDir, filepath.Base(freshDir), legacyDir,
	)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// eagerParseFlags parses the global app flags before calling pb.RootCmd.Execute().
// so we can have all MiraBase flags ready for use on initialization.
func (pb *MiraBase) eagerParseFlags(config *Config) error {
	pb.RootCmd.PersistentFlags().StringVar(
		&pb.dataDirFlag,
		"dir",
		config.DefaultDataDir,
		"the MiraBase data directory",
	)

	pb.RootCmd.PersistentFlags().StringVar(
		&pb.encryptionEnvFlag,
		"encryptionEnv",
		config.DefaultEncryptionEnv,
		"the env variable whose value of 32 characters will be used \nas encryption key for the app settings (default none)",
	)

	pb.RootCmd.PersistentFlags().BoolVar(
		&pb.devFlag,
		"dev",
		config.DefaultDev,
		"enable dev mode, aka. printing logs and sql statements to the console",
	)

	pb.RootCmd.PersistentFlags().IntVar(
		&pb.queryTimeout,
		"queryTimeout",
		int(config.DefaultQueryTimeout.Seconds()),
		"the default SELECT queries timeout in seconds",
	)

	return pb.RootCmd.ParseFlags(os.Args[1:])
}

// skipBootstrap eagerly checks if the app should skip the bootstrap process:
// - already bootstrapped
// - is unknown command
// - is the default help command
// - is the default version command
//
func (pb *MiraBase) skipBootstrap() bool {
	flags := []string{
		"-h",
		"--help",
		"-v",
		"--version",
	}

	if pb.IsBootstrapped() {
		return true // already bootstrapped
	}

	cmd, _, err := pb.RootCmd.Find(os.Args[1:])
	if err != nil {
		return true // unknown command
	}

	for _, arg := range os.Args {
		if !list.ExistInSlice(arg, flags) {
			continue
		}

		// ensure that there is no user defined flag with the same name/shorthand
		trimmed := strings.TrimLeft(arg, "-")
		if len(trimmed) > 1 && cmd.Flags().Lookup(trimmed) == nil {
			return true
		}
		if len(trimmed) == 1 && cmd.Flags().ShorthandLookup(trimmed) == nil {
			return true
		}
	}

	return false
}

// inspectRuntime tries to find the base executable directory and how it was run.
//
// note: we are using os.Args[0] and not os.Executable() since it could
// break existing aliased binaries (eg. the community maintained homebrew package)
func inspectRuntime() (baseDir string, withGoRun bool) {
	if osutils.IsProbablyGoRun() {
		// probably ran with go run
		withGoRun = true
		baseDir, _ = os.Getwd()
	} else {
		// probably ran with go build
		withGoRun = false
		baseDir = filepath.Dir(os.Args[0])
	}
	return
}

var _ io.Writer = (*nopWrite)(nil)

type nopWrite struct{}

func (w *nopWrite) Write(p []byte) (n int, err error) {
	return
}
