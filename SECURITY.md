# Security policy

MiraBase is maintained as an owned product. Report security issues privately to
the MiraBase maintainers through the private reporting channel configured for
this repository or by contacting the project operators directly. Do not open a
public issue for an exploitable vulnerability until the maintainers have had a
reasonable opportunity to investigate and prepare a fix.

A useful report includes:

- the MiraBase version or commit;
- the affected surface (REST API, dashboard, realtime, SQL gateway, import,
  backup, hooks, or another component);
- exact reproduction steps and the smallest proof of concept available;
- the security impact and affected data or privileges;
- any suggested mitigation, if known.

Please do not test vulnerabilities against systems you do not own or do not
have explicit permission to test. Do not include real credentials, private
keys, or personal data unless they are essential and can be transmitted through
an agreed private channel.

The maintainers will acknowledge legitimate reports, investigate, and
coordinate disclosure. Severity is assessed from exploitability and impact in
a supported MiraBase deployment, not solely from a scanner label.

## Supported code

Security fixes are applied to the maintained MiraBase source line in this
repository. Third-party dependencies keep their own upstream security advisories;
license and dependency attributions remain in their respective notices.
