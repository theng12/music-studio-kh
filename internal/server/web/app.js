'use strict';
const $ = id => document.getElementById(id);
let tracks = [], playing = null, profile = null, artworkTrack = null, artworkVersion = 0;
let toastTimer, searchTimer;
const audio = $('audio');
const preferenceKey = 'tuneharbor-appearance';
let preferences = {theme:'dark',accent:'mint',density:'comfortable',layout:'grid'};
try { preferences = {...preferences,...JSON.parse(localStorage.getItem(preferenceKey)||'{}')}; } catch {}
function applyPreferences(){
 let theme=preferences.theme;if(theme==='system')theme=matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';
 document.documentElement.dataset.theme=theme;document.documentElement.dataset.accent=preferences.accent;document.documentElement.dataset.density=preferences.density;
 $('tracks').classList.toggle('list',preferences.layout==='list');
 ['theme','accent','density'].forEach(key=>$(key).value=preferences[key]);
 try{localStorage.setItem(preferenceKey,JSON.stringify(preferences));}catch{}
}
applyPreferences();matchMedia('(prefers-color-scheme: dark)').addEventListener('change',applyPreferences);
function toast(message){$('toast').textContent=message;$('toast').hidden=false;clearTimeout(toastTimer);toastTimer=setTimeout(()=>$('toast').hidden=true,6000);}
function showLogin(){ $('login-screen').hidden=false;$('app').hidden=true;audio.pause(); }
async function api(path,options={}){
 const response=await fetch(path,options);let result;try{result=await response.json();}catch{result={error:'Unexpected server response'};}
 if(!response.ok){if(response.status===401 && path!=='/api/login')showLogin();throw new Error(result.error||'Request failed');}return result;
}
function imageFor(id){const image=document.createElement('img');image.src='/api/artwork/'+encodeURIComponent(id)+'?v='+artworkVersion;image.alt='';image.loading='lazy';image.addEventListener('error',()=>image.remove());return image;}
async function openLibrary(me){profile=me;$('login-screen').hidden=true;$('app').hidden=false;$('password').value='';$('profile-name').textContent=me.name;$('settings-profile').textContent=me.name;$('settings-username').textContent=me.username;$('settings-address').textContent=location.origin;$('settings-access').textContent=me.lan?'Local network enabled':'This computer only';$('settings-folder').textContent=me.library;await loadTracks();}
async function loadTracks(){tracks=await api('/api/tracks?q='+encodeURIComponent($('search').value));renderTracks();}
function renderTracks(){
 $('track-count').textContent=tracks.length;$('empty-library').hidden=tracks.length>0;$('tracks').replaceChildren();
 $('collection-description').textContent=tracks.length===1?'One song. A good beginning.':tracks.length+' songs, ready to listen.';
 if(!tracks.length&&$('search').value){$('empty-library').querySelector('h3').textContent='No songs found';$('empty-library').querySelector('p').textContent='Try another title, artist, or album.';}else{$('empty-library').querySelector('h3').textContent='Your next favorite starts here';$('empty-library').querySelector('p').textContent='Drop music files here, or choose files to add to your library.';}
 for(const track of tracks){
  const card=document.createElement('article');card.className='track-card';
  const cover=document.createElement('button');cover.className='cover-button';cover.setAttribute('aria-label','Play '+track.title);cover.append(imageFor(track.id));
  const play=document.createElement('span');play.className='cover-play';play.textContent='▶';cover.append(play);cover.addEventListener('click',()=>playTrack(track));
  const detail=document.createElement('div');detail.className='track-detail';const text=document.createElement('div');const title=document.createElement('strong');title.textContent=track.title;const artist=document.createElement('p');artist.textContent=track.artist+' · '+track.album;text.append(title,artist);
  const edit=document.createElement('button');edit.className='icon-button';edit.textContent='⋯';edit.setAttribute('aria-label','Edit artwork for '+track.title);edit.addEventListener('click',()=>{artworkTrack=track;$('artwork-title').textContent=track.title;$('artwork-file').value='';$('artwork-dialog').showModal();});detail.append(text,edit);card.append(cover,detail);$('tracks').append(card);
 }
}
async function playTrack(track){playing=track;$('playing-title').textContent=track.title;$('playing-artist').textContent=track.artist;$('playing-cover').replaceChildren(imageFor(track.id));audio.src='/api/stream/'+encodeURIComponent(track.id);try{await audio.play();}catch{toast('Could not play this file. The browser may not support its audio format.');}}
function skip(offset){if(!tracks.length)return;let index=tracks.findIndex(t=>t.id===playing?.id);index=index<0?0:(index+offset+tracks.length)%tracks.length;playTrack(tracks[index]);}
$('play-pause').addEventListener('click',async()=>{if(!playing){if(tracks[0])playTrack(tracks[0]);return;}if(audio.paused){try{await audio.play();}catch{toast('Playback unavailable. Try another song.');}}else audio.pause();});
$('previous').addEventListener('click',()=>skip(-1));$('next').addEventListener('click',()=>skip(1));audio.addEventListener('ended',()=>skip(1));audio.addEventListener('play',()=>$('play-pause').textContent='Ⅱ');audio.addEventListener('pause',()=>$('play-pause').textContent='▶');
function time(value){if(!Number.isFinite(value))return '0:00';return Math.floor(value/60)+':'+String(Math.floor(value%60)).padStart(2,'0');}
audio.addEventListener('timeupdate',()=>{$('elapsed').textContent=time(audio.currentTime);$('duration').textContent=time(audio.duration);$('seek').value=Number.isFinite(audio.duration)&&audio.duration>0?audio.currentTime/audio.duration*100:0;});$('seek').addEventListener('input',()=>{if(Number.isFinite(audio.duration))audio.currentTime=Number($('seek').value)/100*audio.duration;});audio.volume=.8;$('volume').addEventListener('input',()=>audio.volume=Number($('volume').value));audio.addEventListener('error',()=>toast('Song unavailable or unsupported by this browser. Rescan the library or try another file.'));
$('login-form').addEventListener('submit',async event=>{event.preventDefault();const button=event.currentTarget.querySelector('button');button.disabled=true;$('login-error').textContent='';try{await openLibrary(await api('/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:$('username').value,password:$('password').value})}));}catch(error){$('login-error').textContent=error.message;}finally{button.disabled=false;}});
async function signOut(){try{await api('/api/logout',{method:'POST'});showLogin();}catch(error){toast(error.message);}}
$('logout').addEventListener('click',signOut);$('settings-logout').addEventListener('click',signOut);
$('search').addEventListener('input',()=>{clearTimeout(searchTimer);searchTimer=setTimeout(()=>loadTracks().catch(error=>toast(error.message)),180);});
$('rescan').addEventListener('click',async()=>{const button=$('rescan');button.disabled=true;try{await api('/api/scan',{method:'POST'});await loadTracks();toast('Library refreshed.');}catch(error){toast(error.message);}finally{button.disabled=false;}});
function selectFiles(){$('music-files').click();}$('choose-music').addEventListener('click',selectFiles);$('empty-add').addEventListener('click',selectFiles);
async function importFiles(files){let added=0;for(const file of files){const form=new FormData();form.append('file',file);toast('Importing '+file.name+'…');try{await api('/api/import',{method:'POST',body:form});added++;}catch(error){toast(file.name+': '+error.message);}}await loadTracks();if(added)toast(added+' song'+(added===1?'':'s')+' added to your library.');}
$('music-files').addEventListener('change',async event=>{try{await importFiles([...event.target.files]);}catch(error){toast(error.message);}event.target.value='';});
const drop=$('drop-zone');['dragenter','dragover'].forEach(name=>drop.addEventListener(name,event=>{event.preventDefault();drop.classList.add('dragging');}));drop.addEventListener('dragleave',()=>drop.classList.remove('dragging'));drop.addEventListener('drop',event=>{event.preventDefault();drop.classList.remove('dragging');if(event.dataTransfer.files.length)importFiles([...event.dataTransfer.files]).catch(error=>toast(error.message));else toast('Link downloads are coming in the next milestone. Choose audio files for now.');});document.addEventListener('dragover',event=>event.preventDefault());document.addEventListener('drop',event=>event.preventDefault());
$('artwork-file').addEventListener('change',async event=>{const file=event.target.files[0];if(!file||!artworkTrack)return;try{await api('/api/artwork/'+artworkTrack.id,{method:'PUT',body:file});artworkVersion++;renderTracks();if(playing?.id===artworkTrack.id)$('playing-cover').replaceChildren(imageFor(playing.id));$('artwork-dialog').close();toast('Artwork updated.');}catch(error){toast(error.message);}});
$('reset-artwork').addEventListener('click',async()=>{if(!artworkTrack)return;try{await api('/api/artwork/'+artworkTrack.id,{method:'DELETE'});artworkVersion++;renderTracks();if(playing?.id===artworkTrack.id)$('playing-cover').replaceChildren(imageFor(playing.id));$('artwork-dialog').close();toast('Original artwork restored.');}catch(error){toast(error.message);}});
for(const name of ['theme','accent','density'])$(name).addEventListener('change',event=>{preferences[name]=event.target.value;applyPreferences();});$('layout-toggle').addEventListener('click',()=>{preferences.layout=preferences.layout==='grid'?'list':'grid';applyPreferences();});
function navigate(settings){$('settings-page').hidden=!settings;$('library-page').hidden=settings;$('settings-nav').classList.toggle('active',settings);$('library-nav').classList.toggle('active',!settings);$('page-title').textContent=settings?'Your space.':'Music library.';}
$('settings-nav').addEventListener('click',()=>navigate(true));$('library-nav').addEventListener('click',()=>navigate(false));
api('/api/me').then(openLibrary).catch(()=>showLogin());
setInterval(()=>{if(profile&&!$('app').hidden&&!$('artwork-dialog').open)loadTracks().catch(()=>{});},20000);
