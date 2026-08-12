# Microsoft Store packaging

ZipClip ships to the Microsoft Store as a packaged desktop app (MSIX),
free. Microsoft signs the package and handles updates, so there is no
code-signing certificate to buy. Not UWP: Go and Fyne cannot target it,
and ZipClip needs to run the user's own yt-dlp and ffmpeg, which the
UWP sandbox forbids. `runFullTrust` in the manifest is the standard,
expected capability for a packaged desktop app.

Unlike SimplyAuto (WiX MSI captured by the MSIX Packaging Tool),
ZipClip is a single portable exe, so the MSIX is packed directly with
`makeappx` — no MSI, no elevated capture step, fully scripted.

## Product identity (Partner Center, fixed for the listing's lifetime)

    Package/Identity/Name       TheSimpleDev.ZipClip
    Package/Identity/Publisher  CN=7BF8A48F-7EC9-41A4-861E-5AC76F45DE76
    PublisherDisplayName        The Simple Dev
    Package Family Name         TheSimpleDev.ZipClip_zvgc1ynszgpq2
    Store ID                    9N694TPBZ0GN

These are baked into `AppxManifest.xml`. Only the version changes per
release, and the build script fills that in.

## Building a release

```powershell
cd desktop
go build -trimpath -ldflags -H=windowsgui -o zipclip.exe .
cd ..
.\installer\build-msix.ps1 -Version 0.1.0
```

Output: `bin\ZipClip-0.1.0.msix`, unsigned (correct for Store upload —
Microsoft signs it during certification).

## Testing a build locally

Unsigned MSIX packages cannot be double-click installed. With
Settings > System > For developers > Developer Mode on, register the
staged layout instead:

```powershell
Add-AppxPackage -Register .\bin\msix-layout\AppxManifest.xml
```

Verify before submitting:

- App launches from the Start Menu entry "ZipClip".
- It finds yt-dlp and ffmpeg on PATH (or via configured paths).
- Settings survive an app restart (file writes under AppData are
  virtualized under MSIX — the thing most likely to behave differently).
- Uninstall from the Start Menu is clean.

Remove the test install with `Get-AppxPackage TheSimpleDev.ZipClip | Remove-AppxPackage`.

## Partner Center submission checklist

1. Pricing: Free. Category: Utilities & tools (or Photo & video).
2. Privacy policy URL: https://gozipclip.com/privacy.html
3. Age rating questionnaire: utility, no user-generated content.
4. Upload the `.msix`. `runFullTrust` justification, one sentence:
   "Desktop video utility; requires full trust to run the user's own
   yt-dlp and ffmpeg installations and write video files to
   user-chosen folders."
5. Listing: description from the website, logo, screenshots.
   Note ZipClip does not bundle yt-dlp/ffmpeg — say so in the listing
   description (same framing as the website) so certification testers
   are not surprised by a first-run "not found" state.
6. Submit. Certification usually takes 1-3 business days.
