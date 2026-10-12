#!/usr/bin/env python3
"""Exercise the production sidecar and OS keychain in a desktop D-Bus session."""
import hashlib,json,pathlib,subprocess,tempfile,urllib.request,urllib.parse
root=pathlib.Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory(prefix='tuneharbor-native-') as folder:
 data=pathlib.Path(folder)/'profiles';music=pathlib.Path(folder)/'music';music.mkdir()
 backend=subprocess.Popen([str(root/'src-tauri/target/debug/tuneharbor-server'),'--data',str(data),'desktop','--tools',str(root/'src-tauri/runtime-tools')],stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True)
 sequence=0
 def call(method,params={}):
  global sequence
  sequence+=1;backend.stdin.write(json.dumps({'id':sequence,'method':method,'params':params})+'\n');backend.stdin.flush();reply=json.loads(backend.stdout.readline());assert reply['id']==sequence;assert not reply.get('error'),reply.get('error');return reply.get('result')
 try:
  profile=call('create',dict(name='Native test',library=str(music),username='Owner',password='123456',lan=False,port=0))
  secret=(data/profile['id']/'mobile-secret').read_bytes();assert b'123456' not in secret
  opened=call('start',{'id':profile['id']})
  salt='nativecheck';query=urllib.parse.urlencode(dict(u='oWnEr',t=hashlib.md5(('123456'+salt).encode()).hexdigest(),s=salt,v='1.16.1',c='Amperfy',f='json'))
  with urllib.request.urlopen(opened['url']+'/rest/ping.view?'+query) as response:assert json.load(response)['subsonic-response']['status']=='ok'
  request=urllib.request.Request(opened['url']+'/api/desktop-bootstrap',data=json.dumps({'token':opened['bootstrap']}).encode(),headers={'Content-Type':'application/json'},method='POST')
  with urllib.request.urlopen(request) as response:assert response.status==200;assert response.headers.get('Set-Cookie')
  call('stop',{'id':profile['id']});call('remove',{'id':profile['id'],'confirmed':True});assert music.exists();call('quit');backend.wait(timeout=15)
  print('PASS: native RPC, OS-keychain encrypted credentials, mixed-case Amperfy token login, bootstrap and shutdown')
 finally:
  backend.stdin.close()
  if backend.poll() is None:backend.wait(timeout=15)
