# Updating external tool checkouts (deps + koneko)

`deps` (F2 dependency walker) and `koneko` (F3 viewer) are **external tools** that
live in **sibling checkouts** next to this repository:

| Tool | Sibling checkout | Upstream repo |
|------|------------------|---------------|
| `deps` | `..\deps` | <https://github.com/Fiend3d/deps> |
| `koneko` | `..\koneko` | <https://github.com/Fiend3d/koneko> |

`.\build.ps1 dist` rebuilds both tools from their **current local sources** and
packages the resulting `.exe` files. It **does not fetch or pull Git updates** —
it only builds whatever is already checked out. A missing checkout or a failed
tool build stops packaging.

## Agent requirement

Before any distribution build (`.\build.ps1 dist`), **update `deps` and `koneko`
to the latest upstream version**. Otherwise the release ships stale tools.

For each sibling checkout:

1. If the checkout is missing, clone it from its upstream repo into the sibling
   directory.
2. If it exists, pull the latest changes.

```powershell
# Run from the mc repository directory.
$depsRepo   = "https://github.com/Fiend3d/deps"
$konekoRepo = "https://github.com/Fiend3d/koneko"

foreach ($tool, $repo in @(@("deps", $depsRepo), @("koneko", $konekoRepo))) {
    $dir = Join-Path (Join-Path $PSScriptRoot "..") $tool
    if (Test-Path -LiteralPath (Join-Path $dir ".git")) {
        Push-Location -LiteralPath $dir
        try { git pull } finally { Pop-Location }
    }
    else {
        git clone $repo $dir
    }
}
```

Then run the distribution build, which rebuilds the tools from the updated
sources:

```powershell
.\build.ps1 dist
```

Plain `.\build.ps1` builds only `mc` and does not touch these checkouts.
