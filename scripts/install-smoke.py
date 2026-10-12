#!/usr/bin/env python3
"""Check packaged paths and install a private development package on CI."""
import argparse,pathlib,platform,subprocess,os
parser=argparse.ArgumentParser();parser.add_argument('--target',required=True);args=parser.parse_args();root=pathlib.Path(__file__).resolve().parents[1];bundle=root/'src-tauri/target'/args.target/'release/bundle'
def run(*command):return subprocess.check_output(command,text=True,stderr=subprocess.STDOUT)
if platform.system()=='Linux':
 package=next(bundle.rglob('*.deb'));contents=run('dpkg-deb','--contents',str(package));assert 'tuneharbor-server' in contents and 'runtime-tools/ffmpeg' in contents and 'runtime-tools/yt-dlp' in contents
 print(run('sudo','dpkg','-i',str(package)));assert pathlib.Path('/usr/bin/tuneharbor').exists();print(run('sudo','dpkg','--remove','tuneharbor'))
elif platform.system()=='Darwin':
 package=next(bundle.rglob('*.dmg'));mount=root/'artifacts/dmg-mount';mount.mkdir(parents=True,exist_ok=True)
 run('hdiutil','attach','-nobrowse','-mountpoint',str(mount),str(package))
 try:
  app=next(mount.glob('*.app'));assert (app/'Contents/MacOS/tuneharbor-server').exists();ffmpeg=next(app.rglob('runtime-tools/ffmpeg'));print(run(str(ffmpeg),'-version').splitlines()[0]);print(run('codesign','--verify','--deep',str(app)))
 finally:run('hdiutil','detach',str(mount))
else:
 package=next(bundle.rglob('*.exe'));install=root/'artifacts/windows-installed';install.mkdir(parents=True,exist_ok=True)
 # NSIS /D must be the last argument; a custom isolated path is reversible.
 process=subprocess.run(f'"{package}" /S /D={install}',shell=True,timeout=180);assert process.returncode==0
 assert (install/'TuneHarbor.exe').exists() or (install/'tuneharbor.exe').exists();assert (install/'tuneharbor-server.exe').exists();ffmpeg=next(install.rglob('ffmpeg.exe'));print(run(str(ffmpeg),'-version').splitlines()[0]);uninstaller=next(install.glob('*ninstall*.exe'));subprocess.run([str(uninstaller),'/S'],check=True,timeout=120)
print('PASS: native package payload and install/removal smoke; GUI and real devices remain separate gates.')
