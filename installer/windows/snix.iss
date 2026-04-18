; snix Windows installer (Inno Setup 6+)
; Bundles: snix.exe + WinDivert.dll + WinDivert64.sys
; Produces: dist/snix-setup.exe
;
; Build locally:
;   iscc.exe installer/windows/snix.iss
;
; Build in CI:
;   choco install innosetup -y
;   iscc.exe installer/windows/snix.iss

#ifndef MyAppVersion
  #define MyAppVersion "0.0.0-dev"
#endif

#define MyAppName      "snix"
#define MyAppPublisher "SamNet-dev"
#define MyAppURL       "https://snix.sh"
#define MyAppExeName   "snix.exe"

[Setup]
; Randomly generated once; DO NOT change between releases or Windows will
; treat upgrades as separate products. Regenerate only for a major fork.
AppId={{8B77BBB0-6687-46AF-9520-D43412081DB6}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
UninstallDisplayIcon={app}\{#MyAppExeName}
Compression=lzma
SolidCompression=yes
OutputDir=..\..\dist
OutputBaseFilename=snix-setup-{#MyAppVersion}
WizardStyle=modern
; snix.exe always needs Administrator; install privilege mirrors that.
PrivilegesRequired=admin
; 64-bit install since we ship x64-only for now.
ArchitecturesInstallIn64BitMode=x64

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked
Name: "startmenushortcut"; Description: "Create a &Start Menu shortcut"; GroupDescription: "Additional icons:"
Name: "runservice"; Description: "&Install and start as a Windows Service (run on boot)"; GroupDescription: "Service:"; Flags: unchecked
Name: "firstruntui"; Description: "&Launch the snix TUI when the installer finishes"; GroupDescription: "Post-install:"

[Files]
; The GitHub Actions release workflow stages these files under dist/ before iscc runs.
Source: "..\..\dist\snix.exe";             DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\WinDivert.dll";        DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\WinDivert64.sys";      DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE";                   DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md";                 DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName} (TUI)";       Filename: "{app}\{#MyAppExeName}"; Parameters: "tui"; Tasks: startmenushortcut
Name: "{group}\{#MyAppName} (wizard)";    Filename: "{app}\{#MyAppExeName}"; Parameters: "init --wizard"; Tasks: startmenushortcut
Name: "{group}\{#MyAppName} Website";     Filename: "{#MyAppURL}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}";       Filename: "{app}\{#MyAppExeName}"; Parameters: "tui"; Tasks: desktopicon

[Run]
; Install / start Windows Service if the user opted in.
Filename: "sc.exe"; Parameters: "create snix binPath= ""\""{app}\{#MyAppExeName}\"" start"" start= auto DisplayName= ""snix SNI-spoof""";  Tasks: runservice; Flags: runhidden
Filename: "sc.exe"; Parameters: "description snix ""Cross-platform SNI-spoofing DPI bypass.""";                                           Tasks: runservice; Flags: runhidden
Filename: "sc.exe"; Parameters: "start snix";                                                                                              Tasks: runservice; Flags: runhidden
; Launch the TUI.
Filename: "{app}\{#MyAppExeName}"; Parameters: "tui"; Description: "Launch snix TUI"; Tasks: firstruntui; Flags: postinstall skipifsilent nowait

[UninstallRun]
; Clean up the service on uninstall.
Filename: "sc.exe"; Parameters: "stop snix";    Flags: runhidden; RunOnceId: "StopSnixSvc"
Filename: "sc.exe"; Parameters: "delete snix";  Flags: runhidden; RunOnceId: "DeleteSnixSvc"

[Code]
procedure InitializeWizard();
begin
  WizardForm.WelcomeLabel2.Caption :=
    'This will install snix v{#MyAppVersion} on your computer.' + #13#10 + #13#10 +
    'snix needs Administrator privileges to capture packets.' + #13#10 +
    'WinDivert.dll and WinDivert64.sys are installed alongside snix.exe.' + #13#10 + #13#10 +
    'Click Next to continue, or Cancel to exit.';
end;
