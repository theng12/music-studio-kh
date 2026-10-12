#!/usr/bin/env python3
"""Prepare verified runtime executables; FFmpeg is built separately from pinned sources."""
import argparse,hashlib,json,os,pathlib,shutil,subprocess,urllib.request,zipfile
parser=argparse.ArgumentParser();parser.add_argument('--target',required=True);parser.add_argument('--output',default='src-tauri/runtime-tools');parser.add_argument('--ffmpeg-dir');args=parser.parse_args()
root=pathlib.Path(__file__).resolve().parents[1];manifest=json.loads((root/'packaging/runtime-tools.json').read_text());out=pathlib.Path(args.output).resolve();out.mkdir(parents=True,exist_ok=True);cache=root/'artifacts/tool-cache';cache.mkdir(parents=True,exist_ok=True)
def download(url,digest,name):
 path=cache/name
 if not path.exists() or hashlib.sha256(path.read_bytes()).hexdigest()!=digest:
  temporary=path.with_suffix('.download')
  with urllib.request.urlopen(url,timeout=60) as response,temporary.open('wb') as file:shutil.copyfileobj(response,file)
  if hashlib.sha256(temporary.read_bytes()).hexdigest()!=digest:temporary.unlink();raise RuntimeError('Checksum mismatch for '+name)
  temporary.replace(path)
 return path
exe='.exe' if args.target.startswith('windows') else ''
for tool,key in [('yt-dlp','ytDlp'),('deno','deno')]:
 info=manifest[key];asset,digest=info['assets'][args.target];path=download(info['base']+asset,digest,asset);dest=out/(tool+exe)
 if asset.endswith('.zip'):
  with zipfile.ZipFile(path) as archive:dest.write_bytes(archive.read('deno'+exe))
 else:shutil.copyfile(path,dest)
 dest.chmod(0o755)
for key in ['ffmpeg','lame']:
 info=manifest[key];download(info['url'],info['sha256'],key+'-'+info['version']+'.tar.gz')
if args.ffmpeg_dir:
 source=pathlib.Path(args.ffmpeg_dir)
 for name in ['ffmpeg','ffprobe']:shutil.copy2(source/(name+exe),out/(name+exe))
 for name in ['notices','sources']:
  if (source/name).exists():shutil.copytree(source/name,out/name,dirs_exist_ok=True)
notices=out/'notices';notices.mkdir(exist_ok=True)
(notices/'DEPENDENCIES.txt').write_text('yt-dlp standalone 2026.08.19: GPL-3.0-or-later; upstream source: https://github.com/yt-dlp/yt-dlp/tree/2026.08.19\nDeno 2.9.7: MIT with third-party notices; source: https://github.com/denoland/deno/tree/v2.9.7\nFFmpeg 9.0.2: LGPL-2.1-or-later, built without GPL/nonfree codecs or networking. Source archives and build configuration accompany this package. LAME 3.100: LGPL.\nReview the full dependency/source obligations before public distribution.\n')
with urllib.request.urlopen('https://raw.githubusercontent.com/denoland/deno/v2.9.7/LICENSE.md',timeout=30) as response:(notices/'Deno-LICENSE-MIT.txt').write_bytes(response.read())
(out/'provenance.json').write_text(json.dumps(manifest,indent=2)+'\n')
print('Verified runtime tools prepared for '+args.target)
