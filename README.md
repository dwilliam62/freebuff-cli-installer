<div align="center">

# 🚀 Freebuff CLI Installer

**One command to get [Freebuff](https://freebuff.com) — the AI coding agent — running on your Linux box.**

<p>
  <img alt="script version" src="https://img.shields.io/badge/script-v0.6-2ea043?style=for-the-badge&logo=gnubash&logoColor=white">
  <img alt="platforms" src="https://img.shields.io/badge/platform-Debian%20%7C%20Ubuntu%20%7C%20Arch%20%7C%20Fedora%20%7C%20Gentoo-1f6feb?style=for-the-badge&logo=linux&logoColor=white">
  <img alt="shell" src="https://img.shields.io/badge/shell-bash-4eaa25?style=for-the-badge&logo=gnubash&logoColor=white">
  <img alt="license" src="https://img.shields.io/badge/license-GPL--3.0-blueviolet?style=for-the-badge&logo=opensourceinitiative&logoColor=white">
</p>

<p>
  <a href="https://freebuff.com"><img alt="Website" src="https://img.shields.io/badge/Website-freebuff.com-8a2be2?style=flat-square&logo=googlechrome&logoColor=white"></a>
  <a href="https://discord.com/invite/yXG3w7wxfs"><img alt="Discord" src="https://img.shields.io/badge/Discord-Join%20the%20community-5865f2?style=flat-square&logo=discord&logoColor=white"></a>
  <a href="https://github.com/dwilliam62/FreeBuff-cli-installer"><img alt="GitHub" src="https://img.shields.io/badge/GitHub-FreeBuff--cli--installer-181717?style=flat-square&logo=github&logoColor=white"></a>
</p>

</div>

---

## 🎯 Purpose

`freebuff-cli.sh` is a single-file, dependency-light installer and launcher for the
[Freebuff](https://freebuff.com) CLI. It takes a fresh machine to a working agent in one step:

| Step | What happens |
|:--|:--|
| 1️⃣ | Checks that Node.js and npm exist and are new enough (default **≥ v18**) |
| 2️⃣ | Installs Node.js with your distro's package manager if it is missing or too old |
| 3️⃣ | Falls back to **NodeSource** (Debian/Ubuntu, Fedora/RHEL) when the distro build is stale |
| 4️⃣ | Falls back to **nvm** (no root needed) when neither of the above is possible — also the route taken on Gentoo, which has no NodeSource repo |
| 5️⃣ | Installs `freebuff` globally, or runs it on demand via `npx` if npm's prefix is read-only |
| 6️⃣ | Reports installed vs. latest freebuff version, then launches `npx --yes freebuff` |

Everything that changes the system can be previewed first with `--dry-run`, and the whole
toolchain can be inspected without installing anything with `--check`.

> 💡 **Why a script?** Freebuff ships on npm as a thin launcher, so `npx freebuff` is all you
> really need — the tricky part is having a *current* Node.js on a distro that may ship an
> ancient one. This script handles that part, with explicit fallbacks and no silent surprises.

---

## 📋 Supported systems

<div align="left">

| Family | Distributions | Installer used |
|:--|:--|:--|
| 🟠 Debian family | Debian, Ubuntu, Mint, Pop!_OS | `apt-get` → NodeSource `deb` |
| 🔵 Arch family | Arch, Manjaro, EndeavourOS | `pacman` |
| 🎩 Fedora family | Fedora, RHEL, CentOS | `dnf` → NodeSource `rpm` |
| 🧩 Gentoo family | Gentoo | `emerge` (`dev-lang/nodejs`, npm bundled) |
| 🟣 **NixOS** | NixOS | ❌ refused by the script — use [`nixos/`](#-nixos-integration) |
| ⚪ Anything else | Alpine, openSUSE, … | ❌ refused — `Unsupported distro` |

</div>

On an unsupported system the script exits **before touching anything**:

```console
$ ./freebuff-cli.sh
Error: Unsupported distro: NixOS 26.11 (Zokor)
Error: This script supports Debian/Ubuntu (apt), Arch (pacman), Fedora/RHEL (dnf)
Error: and Gentoo (emerge) based systems only.
Error: Use --force to attempt it anyway (not recommended).
```

**Gentoo notes** 🧩 — the script runs `emerge --ask=n --noreplace dev-lang/nodejs`, which
pulls npm and npx in as part of the package (the default `npm` USE flag). It never edits your
world file. Gentoo is a source-based, rolling distribution with no **NodeSource** repository,
so if `::gentoo` carries a `dev-lang/nodejs` older than the minimum — or the package is masked
or keyword-gated — the script goes straight to the **nvm** fallback (`~/.nvm`, no root) rather
than failing. npm's global prefix (`/usr/lib/node_modules`) is root-owned, so expect the
global install to warn and the launch to continue through `npx --yes`; that is normal on Gentoo.

`--force` overrides the check for testing, with a warning. It is not meant for daily use.

---

## ⚡ Quick start

```bash
git clone https://github.com/dwilliam62/FreeBuff-cli-installer.git
cd FreeBuff-cli-installer
chmod +x freebuff-cli.sh
./freebuff-cli.sh
```

Preview first if you prefer:

```bash
./freebuff-cli.sh --check     # what is installed, what is missing (changes nothing)
./freebuff-cli.sh --dry-run   # the exact commands a real run would execute
```

---

## 📥 Installation

### Option A — run it from the clone

```bash
~/FreeBuff-cli-installer/freebuff-cli.sh
```

### Option B — put it on your `PATH` (recommended)

```bash
./freebuff-cli.sh --install          # copies to /usr/local/bin/freebuff-cli.sh
freebuff-cli.sh                      # now available as a command
```

A plain `./freebuff-cli.sh` also copies itself to `$INSTALL_PATH` on a best-effort basis, so
you do not strictly need `--install`; it never fails the launch, and says
`Not installed to … — run '… --install'` when the copy is not possible (no sudo, missing or
unwritable target). A piped `curl … | bash` run is skipped silently, because `$0` is not the
script file to copy.

| Behaviour | Detail |
|:--|:--|
| 🔁 Self-install | a normal run copies the script to `$INSTALL_PATH` when it can; `--install` is the explicit, checked form |
| 🔐 Elevation | uses `sudo` only when the target directory is not writable |
| 🚫 No surprises | **refuses to create a missing directory** — it never fabricates `/usr/local/bin` |
| 🎯 Custom target | `FREEBUFF_INSTALL_PATH=$HOME/.local/bin/freebuff-cli.sh ./freebuff-cli.sh --install` |
| 🧹 Removal | `./freebuff-cli.sh --remove` (or `-r`); `--uninstall` still works as an alias |

### Option C — NixOS

Use the module in [`nixos/`](#-nixos-integration). Do **not** run this script on NixOS.

### Option D — no install at all

```bash
npx --yes freebuff
```

The script is a convenience wrapper around this; if Node.js is already current, `npx` alone is enough.

---

## 🎛️ Usage

```console
Usage: freebuff-cli.sh v0.6 [OPTIONS] [--] [ARGS...]
```

<div align="left">

| Flag | Description |
|:--|:--|
| `-u`, `--upgrade` | Update freebuff to the latest published version before launching |
| `-n`, `--dry-run` | Print every install/launch command, change nothing |
| `--nvm` | Install Node.js with nvm (no root) instead of system packages |
| `-c`, `--check` | Read-only report: distro, Node, npm, npx, freebuff versions (exit `1` if not ready) |
| `--install` | Copy the script to `$INSTALL_PATH` and exit |
| `-r`, `--remove` | Remove the installed copy and exit |
| `--force` | Run on an unsupported distro (testing escape hatch) |
| `--changes` | Print what the script does plus its change history |
| `-v`, `--version` | Print the script version |
| `-h`, `--help` | Show help |

</div>

Any argument the script does not recognise is forwarded to `freebuff`, and `--` stops option parsing:

```bash
./freebuff-cli.sh -- --help          # freebuff's own help
./freebuff-cli.sh --upgrade -- --flag
```

### Environment variables

| Variable | Default | Purpose |
|:--|:--|:--|
| `FREEBUFF_MIN_NODE_MAJOR` | `18` | Minimum acceptable Node.js major version |
| `FREEBUFF_NODESOURCE_MAJOR` | `22` | Node.js major installed by the NodeSource fallback |
| `FREEBUFF_NVM_VERSION` | `v0.40.1` | nvm tag installed by the no-root fallback |
| `FREEBUFF_INSTALL_PATH` | `/usr/local/bin/freebuff-cli.sh` | Target for `--install` / `--remove` |

### Examples

```bash
./freebuff-cli.sh                        # ensure everything, then launch freebuff
./freebuff-cli.sh --check                # status report, exit code 0 = ready
./freebuff-cli.sh --upgrade              # update freebuff, then launch
./freebuff-cli.sh --nvm                  # force the nvm route (no root)
./freebuff-cli.sh --dry-run --upgrade    # show what an upgrade would do
./freebuff-cli.sh --install              # put it on PATH
./freebuff-cli.sh --remove               # take it off PATH
```

---

## 🧩 How it works

```text
Node.js missing or < min ──► distro package manager ──► NodeSource ──► nvm
                                        (fallback)        (fallback)
freebuff missing ──► npm install -g freebuff  ──► or npx --yes freebuff
launch ──► exec npx --yes freebuff "$@"
```

- ✅ **Idempotent** — re-running never reinstalls what is already current.
- ✅ **Read-only npm prefixes handled** — a failed `npm install -g` (root-owned prefix,
  immutable distro) downgrades to `npx --yes`, which fetches into the npm cache instead.
- ✅ **Offline-tolerant** — an unreachable registry prints "unknown" rather than failing.
- ✅ **Nothing hidden** — every mutating step is one `--dry-run` away from being printed.

---

## 🐧 NixOS integration

NixOS is deliberately **not** supported by the bash script: `/usr/local/bin` is not part of
the NixOS filesystem layout, global npm installs fight the immutable store, and the freebuff
binary needs a dynamic loader NixOS does not expose by default. Use the Nix module instead.

Pointed at [`nixos/freebuff-cli.nix`](nixos/freebuff-cli.nix):

<div align="left">

| Aspect | Detail |
|:--|:--|
| 🧩 Type | Home Manager module (`home.packages`) |
| ⚙️ Creates | `freebuff-cli` — a wrapper around `npx --yes freebuff "$@"` |
| 📦 Provides | Node.js from `pkgs.nodejs`, added to `PATH` regardless of your shell |
| 🛡️ Requires | `programs.nix-ld.enable = true;` so the downloaded native binary can start |
| 🏷️ Why `freebuff-cli` | a global/desktop build named `freebuff` can shadow the CLI; the wrapper always resolves the npm package |

</div>

### Enable it

```nix
# home.nix / home-manager configuration
{ pkgs, ... }: {
  imports = [ ./FreeBuff-cli-installer/nixos/freebuff-cli.nix ]; # or copy into modules/home/cli/
}
```

### nix-ld is mandatory

`freebuff` is published as a thin launcher that downloads a platform binary on first run.
That binary is a generic dynamically linked ELF, so NixOS' `stub-ld` refuses to start it
with *"Could not start dynamically linked executable"* unless `nix-ld` is enabled:

```nix
# modules/packages.nix
programs.nix-ld.enable = true;
```

The wrapper checks for `/run/current-system/sw/share/nix-ld/lib/ld.so` and fails fast with
instructions if it is missing.

### Apply

```bash
home-manager switch --flake ~/NixOS-Hyprland/#<host>
# or
sudo nixos-rebuild switch --flake ~/NixOS-Hyprland/#<host>
```

Then run:

```bash
freebuff-cli
```

> 📝 The same module also lives in `NixOS-Hyprland/modules/home/cli/freebuff-cli.nix`; this
> repository keeps the standalone copy so it can be reused in other flakes.

---

## 🩺 Troubleshooting

<div align="left">

| Symptom | Fix |
|:--|:--|
| `Unsupported distro: …` | Use a supported distro, or `--force` to attempt it anyway |
| `/usr/local/bin does not exist — refusing to create it` | `FREEBUFF_INSTALL_PATH=$HOME/.local/bin/freebuff-cli.sh ./freebuff-cli.sh --install` |
| `… is not in your PATH` | Add the directory to `PATH` (e.g. `~/.local/bin`) |
| `npm install -g` warns and falls back to npx | Expected on read-only npm prefixes; the launch still works |
| `Node.js vX is older than the required v18` | The script tries NodeSource, then nvm; or force it with `--nvm` |
| `emerge` is blocked by a mask / keyword / USE change (Gentoo) | Unmask it, or skip emerge entirely with `./freebuff-cli.sh --nvm` |
| `emerge: command not found` | You are not on Gentoo, or `sys-apps/portage` is not in `PATH` |
| `Could not start dynamically linked executable` (NixOS) | `programs.nix-ld.enable = true;` and rebuild |
| `Failed to download …` | Ensure `curl` or `wget` is installed |
| freebuff launches a different build than expected | Use the `freebuff-cli` wrapper (see NixOS section) |

</div>

---

## 🤝 Community & links

<div align="center">

|  |  |
|:--|:--|
| 🌐 **Website** | <https://freebuff.com> |
| 💬 **Discord** | <https://discord.com/invite/yXG3w7wxfs> |
| 🐙 **Repository** | <https://github.com/dwilliam62/FreeBuff-cli-installer> |
| 📜 **Changelog** | [CHANGELOG.md](CHANGELOG.md) |

</div>

---

## 📄 License

GNU General Public License v3.0 — see [LICENSE](LICENSE).

<div align="center">

**Happy coding!** ✨

</div>
