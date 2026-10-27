Unicode true
!include "MUI2.nsh"
!include "x64.nsh"
!include "LogicLib.nsh"

!define APP_NAME "V.W.R.D"
!define APP_VERSION "1.0.2"
!define PROJECT_DIR ".."
!define APP_KEY "Software\VWRD"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\V.W.R.D"
!ifndef GUI_BYTES
  !error "GUI_BYTES must be supplied by build-installer.ps1"
!endif
!ifndef MCP_BYTES
  !error "MCP_BYTES must be supplied by build-installer.ps1"
!endif

Name "${APP_NAME} ${APP_VERSION}"
OutFile "${PROJECT_DIR}\VWRD-Setup.exe"
InstallDir "$LOCALAPPDATA\Programs\V.W.R.D"
InstallDirRegKey HKCU "${APP_KEY}" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma
CRCCheck on
Icon "${PROJECT_DIR}\branding\VWRD.ico"
UninstallIcon "${PROJECT_DIR}\branding\VWRD.ico"
VIProductVersion "1.0.2.0"
VIAddVersionKey "ProductName" "V.W.R.D"
VIAddVersionKey "FileDescription" "V.W.R.D 설치 프로그램"
VIAddVersionKey "FileVersion" "1.0.2.0"
VIAddVersionKey "ProductVersion" "${APP_VERSION}"
VIAddVersionKey "LegalCopyright" "Copyright 2026 V.W.R.D"

!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\VWRD.exe"
!define MUI_FINISHPAGE_RUN_TEXT "V.W.R.D 실행"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "Korean"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP|MB_OK "64비트 Windows가 필요합니다."
    Abort
  ${EndIf}
FunctionEnd

Function CheckRunningApp
  check_gui:
  IfFileExists "$INSTDIR\VWRD.exe" 0 check_mcp
  ClearErrors
  FileOpen $0 "$INSTDIR\VWRD.exe" a
  IfErrors gui_busy
  FileClose $0
  check_mcp:
  IfFileExists "$INSTDIR\VWRD-MCP.exe" 0 done
  ClearErrors
  FileOpen $0 "$INSTDIR\VWRD-MCP.exe" a
  IfErrors mcp_busy
  FileClose $0
  done:
  Return
  gui_busy:
  StrCpy $1 "V.W.R.D 앱"
  Goto busy
  mcp_busy:
  StrCpy $1 "V.W.R.D MCP 서버"
  busy:
  IfSilent fail
  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "$1이 실행 중이거나 설치 파일이 잠겨 있습니다.$\r$\n종료한 뒤 다시 시도하세요. 설치는 아직 변경되지 않았습니다." IDRETRY check_gui
  fail:
  SetErrorLevel 32
  Abort
FunctionEnd

Function VerifyInstalledFiles
  ClearErrors
  FileOpen $0 "$INSTDIR\VWRD.exe" r
  IfErrors bad_gui
  FileSeek $0 0 END $1
  FileClose $0
  IntCmp $1 ${GUI_BYTES} check_mcp bad_gui bad_gui
  check_mcp:
  ClearErrors
  FileOpen $0 "$INSTDIR\VWRD-MCP.exe" r
  IfErrors bad_mcp
  FileSeek $0 0 END $1
  FileClose $0
  IntCmp $1 ${MCP_BYTES} done bad_mcp bad_mcp
  done:
  Return
  bad_gui:
  StrCpy $1 "VWRD.exe"
  Goto bad
  bad_mcp:
  StrCpy $1 "VWRD-MCP.exe"
  bad:
  IfSilent fail
  MessageBox MB_OK|MB_ICONSTOP "$1 파일이 제대로 갱신되지 않았습니다. 앱과 MCP 서버를 종료하고 설치를 다시 실행하세요."
  fail:
  SetErrorLevel 33
  Abort
FunctionEnd

Section "V.W.R.D" SecApp
  SectionIn RO
  SetShellVarContext current
  SetOutPath "$INSTDIR"
  Call CheckRunningApp
  SetOverwrite on
  File "${PROJECT_DIR}\VWRD-release\VWRD.exe"
  File "${PROJECT_DIR}\VWRD-release\VWRD-MCP.exe"
  Call VerifyInstalledFiles
  File "${PROJECT_DIR}\README.md"
  File "${PROJECT_DIR}\LICENSE"
  SetOutPath "$INSTDIR\branding"
  File "${PROJECT_DIR}\branding\VWRD-logo.svg"
  File "${PROJECT_DIR}\branding\VWRD-logo.png"
  File "${PROJECT_DIR}\branding\VWRD.ico"
  SetOutPath "$INSTDIR"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\V.W.R.D"
  CreateShortcut "$SMPROGRAMS\V.W.R.D\V.W.R.D.lnk" "$INSTDIR\VWRD.exe"
  WriteRegStr HKCU "${APP_KEY}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "V.W.R.D"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "V.W.R.D"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\VWRD.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '$\"$INSTDIR\Uninstall.exe$\" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Section /o "바탕화면 바로가기" SecDesktop
  SetShellVarContext current
  CreateShortcut "$DESKTOP\V.W.R.D.lnk" "$INSTDIR\VWRD.exe"
SectionEnd

Section "Uninstall"
  SetShellVarContext current
  Delete "$DESKTOP\V.W.R.D.lnk"
  Delete "$SMPROGRAMS\V.W.R.D\V.W.R.D.lnk"
  RMDir "$SMPROGRAMS\V.W.R.D"
  Delete "$INSTDIR\VWRD.exe"
  Delete "$INSTDIR\VWRD-MCP.exe"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\branding\VWRD-logo.svg"
  Delete "$INSTDIR\branding\VWRD-logo.png"
  Delete "$INSTDIR\branding\VWRD.ico"
  RMDir "$INSTDIR\branding"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  DeleteRegKey HKCU "${APP_KEY}"
SectionEnd
