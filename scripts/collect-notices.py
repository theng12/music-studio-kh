#!/usr/bin/env python3
"""Collect dependency manifests and available license/notice texts for private builds."""
import argparse,json,pathlib,subprocess,shutil
root=pathlib.Path(__file__).resolve().parents[1];parser=argparse.ArgumentParser();parser.add_argument('--output',default='src-tauri/runtime-tools/notices/dependencies');args=parser.parse_args();out=(root/args.output).resolve();out.mkdir(parents=True,exist_ok=True)
def copy_texts(folder,label):
 destination=out/label.replace('/','_').replace('@','_').replace(':','_');destination.mkdir(exist_ok=True)
 for path in folder.iterdir():
  if path.is_file() and path.name.upper().startswith(('LICENSE','LICENCE','COPYING','NOTICE','AUTHORS')) and path.stat().st_size<2<<20:shutil.copyfile(path,destination/path.name)
cargo=json.loads(subprocess.check_output(['cargo','metadata','--locked','--format-version','1','--manifest-path',str(root/'src-tauri/Cargo.toml')],text=True));records=[]
for package in cargo['packages']:
 if package['name']=='tuneharbor':continue
 records.append(dict(ecosystem='Rust',name=package['name'],version=package['version'],license=package['license'],source=package['source']))
 copy_texts(pathlib.Path(package['manifest_path']).parent,'rust_'+package['name']+'_'+package['version'])
lock=json.loads((root/'package-lock.json').read_text())
for name,package in lock['packages'].items():
 if not name:continue
 folder=root/name
 if not folder.exists():continue
 records.append(dict(ecosystem='npm',name=name,version=package.get('version'),license=package.get('license'),dev=package.get('dev',False)))
 copy_texts(folder,'npm_'+name.removeprefix('node_modules/'))
raw=subprocess.check_output(['go','list','-m','-json','all'],cwd=root,text=True);decoder=json.JSONDecoder()
while raw.strip():
 package,length=decoder.raw_decode(raw.lstrip());raw=raw.lstrip()[length:]
 if package.get('Main'):continue
 records.append(dict(ecosystem='Go',name=package['Path'],version=package.get('Version')))
 if package.get('Dir'):copy_texts(pathlib.Path(package['Dir']),'go_'+package['Path']+'_'+package.get('Version',''))
(out/'DEPENDENCY-MANIFEST.json').write_text(json.dumps(records,indent=2)+'\n');print('Collected dependency manifest and available license texts. Full redistribution audit remains a release gate.')
