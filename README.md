# tw-origin

Where does your distro come from? For every installed package, tw-origin
finds the host the upstream source is fetched from, who operates that host,
and in which country. Sovereignty talk is about clouds; the dependency layer
underneath is rarely measured. openSUSE makes it measurable: every RPM
carries `%{DISTURL}`, which names the exact source revision it was built
from, and the Open Build Service serves those sources to anyone.

```
$ tw-origin scan --no-table          # Tumbleweed container, aarch64, vim git python3 curl nodejs installed
tw-origin: 148 installed packages, 108 source packages; reading specs from https://api.opensuse.org/public (8 workers, cache /root/.cache/tw-origin)
  fetch 108/108
  resolve 37/37
  done in 12s: 0 from cache, 228 requests, 0 retries

148 installed packages built from 108 source packages; sources fetched from 37 hosts run by 28 organisations in 5 countries: 40% GitHub (Microsoft) (US), 13% Free Software Foundation (US), 7% g10 Code (DE), 3% GNOME Foundation (US), 3% Linux Foundation (US); 6 packages without an upstream URL.

by host                           src pkgs  share  installed  operator                                   cc  basis
github.com                        43        40%    62         GitHub (Microsoft)                         US  known
ftp.gnu.org                       12        11%    15         Free Software Foundation                   US  known
gnupg.org                         5         5%     5          g10 Code                                   DE  known
zlib.net                          1         1%     1          A2 Hosting                                 US  net
...                               (37 hosts)
(no upstream URL)                 6         6%

by country                 src pkgs  share  installed
US  United States          88        81%    126
DE  Germany                10        9%     10
SE  Sweden                 2         2%     2
BR  Brazil                 1         1%     1
EU  European Union (RIPE)  1         1%     1

found by: Source URL in the spec 79, _service 12, URL tag only 11, no URL 6
```

The per-package table follows (package, source package, host, operator,
country, how found). `tw-origin package zypper` shows one package: spec URL
on OBS, every Source line raw and expanded, `_service` entries, the origin.

## How it works

1. `rpm -qa` gives NAME, SOURCERPM and DISTURL; binaries are grouped by
   source package (`obs://build.opensuse.org/openSUSE:Factory:ARM/standard/<srcmd5>-bash`).
2. The spec and `_service` of exactly that revision are read anonymously from
   `https://api.opensuse.org/public/source/<project>/<pkg>/<file>?rev=<srcmd5>`
   (8 parallel requests, retries with backoff, cached under
   `$XDG_CACHE_HOME/tw-origin/` by srcmd5, so a second run makes no requests).
3. `Source:` lines are expanded with the spec's own `%define`s, `%{name}`,
   `%{version}`, `%{url}` and the multibuild flavor; `%if` is evaluated where
   possible. The origin is Source0's host, else the repository an
   `obs_scm`/`tar_scm` service clones, else another Source URL, else the
   `URL:` tag (marked "URL tag", it is the home page, not the tarball).
4. Hosts are attributed with a built-in table (forges, registries,
   foundations); anything else is resolved and looked up in the iptoasn.com
   database, marked `net`: that is the network's country of registration,
   not a legal seat.

Caveats, stated plainly: a mirror of a tarball is not its author (ftp.gnu.org
hosts code written everywhere); a source service may fetch from a URL that
differs from where the project lives; `%if` branches the reader cannot
evaluate are taken as true; shares are of source packages.

## Install and use

```
go install github.com/officialasishkumar/tw-origin@latest   # or dist/tw-origin-linux-{amd64,arm64}
tw-origin scan [--limit N] [--json] [--no-table] [--offline] [--workers 8]
tw-origin package NAME
tw-origin hosts [--country DE]
tw-origin scan --from list.tsv      # list from: rpm -qa --qf '%{NAME}\t%{ARCH}\t%{SOURCERPM}\t%{DISTURL}\n'
```

## Related work

whatsrc.org and osdiff compare shipped sources with upstream; cavil reviews
licences from the same OBS sources; syft/SBOM `externalRefs` carry purls but
not who hosts them. tw-origin only answers the host question.

## License

MIT. The iptoasn.com data is public domain.
