MiraBase Superuser dashboard UI
======================================

This is the MiraBase Superuser dashboard UI (built with Shablon and Vite),
derived from the upstream PocketBase dashboard.

Although it could be used independently, it is intended to be embedded and extended
as part of the MiraBase app executable (hence the `dist` directory and `embed.go` file).

Build
-----

```sh
npm install
npm run build
```

The build output is stored in `dist` and embedded via `embed.go`.
