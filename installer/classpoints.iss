#define MyAppName "班级积分系统"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "ClassPoints"
#define MyAppExeName "classpoints.exe"
#define MyAppIconName "classpoints.ico"

[Setup]
AppId={{B95CB9E7-19C8-4A74-8BB7-4287758166B1}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\ClassPoints
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\release\installer
OutputBaseFilename=ClassPointsSetup-{#MyAppVersion}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\assets\classpoints.ico
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\{#MyAppIconName}
PrivilegesRequired=lowest
[Languages]
Name: "chinesesimp"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加任务："

[Files]
Source: "..\release\classpoints\classpoints.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\release\classpoints\classpoints.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\release\classpoints\使用说明.txt"; DestDir: "{app}"; Flags: ignoreversion

[Dirs]
Name: "{app}\data"
Name: "{app}\data\backups"
Name: "{app}\logs"

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; IconFilename: "{app}\{#MyAppIconName}"
Name: "{group}\卸载 {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; IconFilename: "{app}\{#MyAppIconName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "启动 {#MyAppName}"; Flags: nowait postinstall skipifsilent; WorkingDir: "{app}"

[UninstallDelete]
Type: filesandordirs; Name: "{app}"

[Code]
function InitializeSetup(): Boolean;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{cmd}'), '/C taskkill /IM classpoints.exe /F >NUL 2>NUL', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(800);
  Result := True;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  ResultCode: Integer;
begin
  if CurUninstallStep = usUninstall then
  begin
    Exec(ExpandConstant('{cmd}'), '/C taskkill /IM classpoints.exe /F >NUL 2>NUL', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Sleep(800);
  end;
end;
