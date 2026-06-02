# Windows Commercial Deployment And Signing Plan

Target: PH Trading Windows PC deployment.

## Release Position

- Do not embed raw AIML or Mistral API keys in the executable, resources, installer, or `.env`.
- Provision API keys through `Settings > AI & Intelligence`; non-admin commercial roles receive only the API-key settings surface.
- Keys are stored by the app through the encrypted settings service and are loaded at runtime by Butler/OCR.

## Certificate

Use a production code-signing certificate issued to the company:

- Preferred: EV Code Signing certificate on hardware token or cloud HSM for faster SmartScreen reputation.
- Acceptable: OV Code Signing certificate if EV procurement is not possible before handoff.
- Timestamp authority: use the certificate vendor timestamp URL, or DigiCert/Sectigo equivalent.

## Windows Build

Build on a Windows signing machine, or cross-compile on macOS only for test artifacts.

```powershell
wails build -platform windows/amd64 -clean
```

Expected unsigned artifact:

```text
build\bin\AsymmFlowPHTrading.exe
```

If an installer is produced, sign both the application executable and installer package.

## Signing Commands

Run from a Windows Developer PowerShell with Windows SDK `signtool.exe` available.

```powershell
signtool sign /fd SHA256 /tr "https://timestamp.digicert.com" /td SHA256 /a "build\bin\AsymmFlowPHTrading.exe"
signtool verify /pa /tw /v "build\bin\AsymmFlowPHTrading.exe"
```

For a certificate thumbprint:

```powershell
signtool sign /fd SHA256 /tr "https://timestamp.digicert.com" /td SHA256 /sha1 "<CERT_THUMBPRINT>" "build\bin\AsymmFlowPHTrading.exe"
```

## Commercial Gate

Before delivery to the client Windows PC:

- `go test ./...`
- `npm run check`
- `npm run build`
- Chromium E2E intent suite
- `wails build -platform windows/amd64 -clean`
- `signtool verify /pa /tw /v`
- Fresh Windows install smoke test
- Activate license on the target PC
- Open Settings as sales/operations/staff and confirm only AI/API-key settings are available
- Save AIML and Mistral keys, restart app, then run Butler chat and OCR document analysis
- Generate offer, costing PDF, invoice/proforma, and purchase order; verify prepared-by signature block matches `Signatures.docx`

## Packaging Rules

- Do not ship `.env`, local SQLite databases, backups, logs, screenshots, or test fixtures inside the installer.
- Include only the signed executable, required Wails runtime assets, app icon/resources, and a clean first-run database if explicitly approved.
- API keys must be configured on the installed machine through the restricted Settings flow or by an approved encrypted provisioning step.
