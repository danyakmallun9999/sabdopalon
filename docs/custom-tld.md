# Custom TLDs: why `.localhost` works and your own suffix does not

Sabdopalon serves each site at `<site>.<tld>`, and the TLD is configurable on the
Settings page. This document explains what changing it actually requires, because
the difference between working and "the site is broken" is DNS, not routing.

## The one thing that decides everything

A browser can only send a request once the NAME resolves. Sabdopalon's proxy
routes on the `Host` header (`hostToSite` compares it against the configured
TLD), so routing has never been the problem — an unresolvable name means the
request never arrives, and the browser reports it as "site can't be reached" with
no error anywhere in Sabdopalon.

Measured on a stock Windows 10 install:

| Name | Resolves? |
| --- | --- |
| `localhost` | `::1`, `127.0.0.1` |
| `anything.localhost` | `::1`, `127.0.0.1` |
| `mysite.sabdopalon` | **no** — "No such host is known" |
| `mysite.test` | **no** — "No such host is known" |
| `mysite.dev` | **no** — "No such host is known" |

`localhost` and every `*.localhost` name resolve by themselves: RFC 6761 reserves
the name, and Windows' resolver plus the browsers map it to loopback. That is why
the default TLD needs no configuration at all.

**Any other suffix needs a hosts entry**, and Sabdopalon did not write one — so
changing the TLD produced a site that was configured, routed, and unreachable.

## What Sabdopalon does now

`internal/hostsync` maintains one block in the OS hosts file:

```
# >>> sabdopalon managed >>>
# Written by Sabdopalon so that *.acme resolve to this machine.
# Delete this block to stop managing them; Sabdopalon recreates it on the next start.
127.0.0.1	alpha.acme
127.0.0.1	beta.acme
127.0.0.1	shop.acme
# <<< sabdopalon managed <<<
```

Properties that matter:

- **Everything outside the block is preserved byte for byte** — Laragon, Herd,
  Docker and the user's own entries are untouched.
- **The block is replaced, not appended**, so changing the TLD removes the old
  names instead of accumulating a block per suffix ever tried.
- **A sync that changes nothing writes nothing.** That is what lets a default
  `.localhost` install run without administrator rights: with nothing to manage,
  the file is never opened for writing.
- Which names exist is defined once, in `hostsync.ForInstall`: every site, plus
  every per-site alias from `.sabdopalon.yml`.

It runs at three moments: on start, after a site is created, and when the TLD is
saved on the Settings page — plus on demand from the Synchronize button below.

## The elevated step

On Windows the hosts file belongs to Administrators, so a normal (non-elevated)
process cannot write it. When that happens Sabdopalon says so instead of failing
silently:

```
The new domains could not be added to the hosts file: it needs administrator
rights. Click Synchronize (Administrator) on this page, or run
`sabdopalon hosts:sync` in an elevated terminal; keep the .localhost TLD and no
rights are needed.
```

### From the Settings page

Settings → **Local DNS (hosts file)** is the same report and the same fix, without
a terminal. The card reads `GET /api/hosts`, which never writes anything: the
wanted hostnames, what the managed block maps now, what is missing, what is
stale from an earlier TLD, and the file's path. The report is green when the two
agree.

**Synchronize (Administrator)** posts to `/api/hosts/elevate`, which raises one
UAC prompt (`Start-Process -Verb RunAs`) and runs `hosts:sync` elevated; the page
then polls the report until the block is there. It refuses to prompt when
nothing needs correcting — a dialog that fixes nothing trains people to approve
blindly — and on platforms without an elevation mechanism it returns the
`sudo sabdopalon hosts:sync` instruction instead of a button that cannot work.

The same card matters after creating a site: creation used to end with a silently
discarded sync, so a site on a custom TLD could be "created" and unreachable at
the address the dialog printed. The create job now carries a note naming this
step, so the fix is one click away from where the failure appeared.

### From a terminal

```
sabdopalon hosts:sync          # from an Administrator terminal
```

It prints exactly which names it mapped, or that `.localhost` needs nothing.

## Certificates follow the TLD

The HTTPS listener uses a wildcard certificate for `*.<tld>`, so a changed TLD
needs a new one. When a local CA already exists, Sabdopalon issues
`*.<new-tld>` during the save and tells you to restart so the listener picks it
up. It does **not** create a CA just because you edited the field: a root
certificate is a trust anchor, and that decision belongs on the SSL page.

Until the restart, HTTPS at the new names is served by the old certificate (or
falls back to `localhost.crt`), which reads as a certificate error rather than a
missing one. HTTP works immediately.

## Suffixes to avoid

- `dev` and `app` are **HSTS-preloaded**: browsers refuse plain HTTP for them and
  demand a trusted certificate. A local CA that is not trusted yet therefore
  produces an error page you cannot click through.
- `local` is claimed by mDNS/Bonjour and can be resolved by something else on the
  network before it reaches this machine.
- `test` is the IANA-reserved testing suffix and is a good choice; `acme`,
  `internal`, or your own invented label work equally well.

## Verifying

```
go test ./internal/hostsync/...     # block handling, no-op safety, hostnames
sabdopalon hosts:sync               # what would be mapped (and map it, elevated)
curl -s localhost:9900/api/hosts    # the Settings card's report, read-only
```
