#!/usr/bin/env python3
"""Check packaged paths and install a private development package on CI."""
import argparse,pathlib,platform,subprocess,os
parser=argparse.ArgumentParser();parser.add_argument('--target',required=True);args=parser.parse_args();root=pathlib.Path(__file__).resolve().parents[1];bundle=root/'src-tauri/target'/args.target/'release/bundle'
def run(*command):
 process=subprocess.run(command,capture_output=True,text=True,encoding='utf-8',errors='replace')
 output=process.stdout+process.stderr
 if process.returncode:raise SystemExit(f'FAIL ({process.returncode}): {" ".join(command)}\n{output}')
 return output
if platform.system()=='Linux':
 package=next(bundle.rglob('*.deb'));contents=run('dpkg-deb','--contents',str(package));assert 'tuneharbor-server' in contents and 'runtime-tools/ffmpeg' in contents and 'runtime-tools/yt-dlp' in contents
 # apt resolves declared dependencies (WebKitGTK, keyring) as a user's package manager would; dpkg -i does not.
 print(run('sudo','env','DEBIAN_FRONTEND=noninteractive','apt-get','install','-y',str(package)));assert pathlib.Path('/usr/bin/tuneharbor').exists() and pathlib.Path('/usr/bin/tuneharbor-server').exists()
 print(run('/usr/bin/tuneharbor-server'));name=run('dpkg-deb','--field',str(package),'Package').strip();print(run('sudo','apt-get','remove','-y',name));assert not pathlib.Path('/usr/bin/tuneharbor').exists()
elif platform.system()=='Darwin':
 package=next(bundle.rglob('*.dmg'));mount=root/'artifacts/dmg-mount';mount.mkdir(parents=True,exist_ok=True)
 run('hdiutil','attach','-nobrowse','-mountpoint',str(mount),str(package))
 try:
  app=next(mount.glob('*.app'));assert (app/'Contents/MacOS/tuneharbor-server').exists();ffmpeg=app/'Contents/MacOS/ffmpeg'
  if not ffmpeg.is_file():ffmpeg=next(app.rglob('runtime-tools/ffmpeg'))
  print(run(str(ffmpeg),'-version').splitlines()[0])
  # Check the bundle seal for both ad-hoc CI and locally signed builds, not notarization.
  print(run('codesign','--verify','--deep','--strict','--verbose=2',str(app)))
 finally:run('hdiutil','detach',str(mount))
else:
 package=next(bundle.rglob('*.exe'));install=root/'artifacts/windows-installed';install.mkdir(parents=True,exist_ok=True)
 # NSIS /D must be the last argument; a custom isolated path is reversible.
 process=subprocess.run(f'"{package}" /S /D={install}',shell=True,timeout=180);assert process.returncode==0
 assert (install/'Music Studio KH.exe').exists() or (install/'tuneharbor.exe').exists();assert (install/'tuneharbor-server.exe').exists();ffmpeg=next(install.rglob('ffmpeg.exe'));print(run(str(ffmpeg),'-version').splitlines()[0]);uninstaller=next(install.glob('*ninstall*.exe'));subprocess.run([str(uninstaller),'/S'],check=True,timeout=120)
print('PASS: native package payload and install/removal smoke; GUI and real devices remain separate gates.')
