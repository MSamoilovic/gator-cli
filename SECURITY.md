# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately, through GitHub: open the
[Security tab](https://github.com/MSamoilovic/gator-cli/security) of this
repository and choose **Report a vulnerability**. Do not open a public issue for
anything that could hurt someone running gator.

Include what you did, what happened, and the version (`gator version`). You will
get an answer within a week. Once a fix is released, the advisory is published
with credit to you, unless you would rather stay anonymous.

## Supported versions

Only the latest release gets security fixes. Upgrade with a new archive from the
[releases page](https://github.com/MSamoilovic/gator-cli/releases), or
`go install github.com/MSamoilovic/gator-cli/cmd/gator@latest`, then run
`gator migrate`.

## What is in scope

- **`gator serve`**: anything that lets one account read or change another
  account's data, sign in without the password, register without an invite,
  make the server fetch an address on its own network, or get past the rate
  limits.
- **The client**: anything a feed, an article page or a server can send that
  runs a command, writes the clipboard, opens a non-web link, leaks the saved
  token, or changes what the terminal shows outside gator's own screen.
- **The release**: an archive whose build provenance does not verify, or a
  workflow that exposes a secret.

## What is out of scope

- **Local mode trusts the operating-system user.** Without a server, gator
  talks straight to PostgreSQL. Whoever holds the `db_url` in
  `~/.gatorconfig.json` has the whole database, whatever gator itself checks.
  The admin role is there to catch the wrong person at the keyboard, not to
  stand between the database and someone who can already reach it.
- **TLS.** `gator serve` speaks plain HTTP. Run it behind a reverse proxy that
  terminates TLS, and pass the proxy's address with `--trusted-proxy`.
- Denial of service that needs more traffic than one machine can send.

## Verifying a download

Every release archive and `checksums.txt` carry a signed build-provenance
attestation:

```bash
gh attestation verify gator_<version>_linux_amd64.tar.gz --repo MSamoilovic/gator-cli
```

## If a token is stolen

- **The account owner** signs out every device with `gator logout --all`.
- **The server's operator** does the same for any account with
  `gator admin --revoke-tokens <name>`. That command needs the admin password
  and is written to `gator admin --log`.

A token unused for 90 days expires by itself.
