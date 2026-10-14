#!/usr/bin/env python3
"""Prepare verified runtime executables; FFmpeg is built separately from pinned sources."""
import argparse,hashlib,json,os,pathlib,shutil,subprocess,time,urllib.error,urllib.request,zipfile
parser=argparse.ArgumentParser();parser.add_argument('--target',required=True);parser.add_argument('--output',default='src-tauri/runtime-tools');parser.add_argument('--ffmpeg-dir');parser.add_argument('--sources-only',action='store_true');parser.add_argument('--local',action='store_true',help='personal build with a system FFmpeg; skips FFmpeg/LAME source archives');args=parser.parse_args()
root=pathlib.Path(__file__).resolve().parents[1];manifest=json.loads((root/'packaging/runtime-tools.json').read_text());out=pathlib.Path(args.output).resolve();out.mkdir(parents=True,exist_ok=True);cache=root/'artifacts/tool-cache';cache.mkdir(parents=True,exist_ok=True)
def retrying(action):
 # Upstream mirrors occasionally time out; retry server and network errors.
 for attempt in range(4):
  try:return action()
  except (urllib.error.URLError,TimeoutError,ConnectionError) as error:
   if attempt==3 or isinstance(error,urllib.error.HTTPError) and error.code<500:raise
   time.sleep(2**(attempt+1))
def fetch_to(url,target):
 with urllib.request.urlopen(url,timeout=60) as response,target.open('wb') as file:shutil.copyfileobj(response,file)
def download(url,digest,name):
 path=cache/name
 if not path.exists() or hashlib.sha256(path.read_bytes()).hexdigest()!=digest:
  temporary=path.with_suffix('.download')
  retrying(lambda:fetch_to(url,temporary))
  if hashlib.sha256(temporary.read_bytes()).hexdigest()!=digest:temporary.unlink();raise RuntimeError('Checksum mismatch for '+name)
  temporary.replace(path)
 return path
for key in ([] if args.local else ['ffmpeg','lame']):
 info=manifest[key];download(info['url'],info['sha256'],key+'-'+info['version']+'.tar.gz')
if args.sources_only:print('Verified source archives ready');raise SystemExit(0)
exe='.exe' if args.target.startswith('windows') else ''
for tool,key in [('yt-dlp','ytDlp'),('deno','deno')]:
 info=manifest[key];asset,digest=info['assets'][args.target];path=download(info['base']+asset,digest,asset);dest=out/(tool+exe)
 if asset.endswith('.zip'):
  with zipfile.ZipFile(path) as archive:dest.write_bytes(archive.read('deno'+exe))
 else:shutil.copyfile(path,dest)
 dest.chmod(0o755)
if args.ffmpeg_dir:
 source=pathlib.Path(args.ffmpeg_dir)
 for name in ['ffmpeg','ffprobe']:shutil.copy2(source/(name+exe),out/(name+exe))
 for name in ['notices','sources']:
  if (source/name).exists():shutil.copytree(source/name,out/name,dirs_exist_ok=True)
source=manifest['ytDlp']['source'];archive=download(source['url'],source['sha256'],'yt-dlp-'+manifest['ytDlp']['version']+'.tar.gz');(out/'sources').mkdir(exist_ok=True);shutil.copy2(archive,out/'sources'/archive.name)
notices=out/'notices';notices.mkdir(exist_ok=True)
(notices/'DEPENDENCIES.txt').write_text('yt-dlp standalone 2026.08.19: GPL-3.0-or-later; upstream source: https://github.com/yt-dlp/yt-dlp/tree/2026.08.19\nDeno 2.9.7: MIT with third-party notices; source: https://github.com/denoland/deno/tree/v2.9.7\nFFmpeg 9.0.2: LGPL-2.1-or-later, built without GPL/nonfree codecs or networking. Source archives and build configuration accompany this package. LAME 3.100: LGPL.\nReview the full dependency/source obligations before public distribution.\n')
if args.local:(notices/'DEPENDENCIES.txt').write_text((notices/'DEPENDENCIES.txt').read_text()+'\nPERSONAL BUILD: FFmpeg here is the system copy given with --ffmpeg-dir, not the LGPL build above. Do not redistribute this package.\n')
retrying(lambda:fetch_to('https://raw.githubusercontent.com/denoland/deno/v2.9.7/LICENSE.md',notices/'Deno-LICENSE-MIT.txt'))
shutil.copy2(root/'scripts/build-ffmpeg.sh',out/'sources/build-ffmpeg.sh')
shutil.copy2(root/'packaging/THIRD-PARTY-NOTICES.md',notices/'THIRD-PARTY-NOTICES.md')
(out/'provenance.json').write_text(json.dumps(manifest,indent=2)+'\n')
print('Verified runtime tools prepared for '+args.target)
