build_installer.bat puts atc.exe and launcher.exe here before building
SkyEye-Setup.exe, which carries them (go:embed). The .exe files are not in
git; this file keeps the folder so the package builds without them.
