# histui-git

Development package tracking upstream `main`. The canonical PKGBUILD lives here;
`.SRCINFO` is generated using `makepkg --printsrcinfo`. Copy both files to the
AUR repository when publishing manually.

The dedicated GitHub workflow uses the existing `AUR_KEY` secret and publishes
recipe changes after they merge to main, or on manual dispatch from main.
Ordinary source commits do not need AUR commits: makepkg fetches current main
and calculates `pkgver()` at build time. Do not submit version-only AUR updates.

To verify without installing:

```sh
makepkg --cleanbuild
makepkg --printsrcinfo > .SRCINFO
```

Run from a disposable directory containing a copy of PKGBUILD. Both binaries
use CGO_ENABLED=0 and PIE; the GTK/audio libraries remain runtime dependencies.
Shell completions are generated from the built native CLI, so packaging must
run on the target architecture (x86_64 or aarch64).
