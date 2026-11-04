import {spawn,spawnSync} from 'node:child_process';
import {mkdirSync,openSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {randomBytes} from 'node:crypto';
import net from 'node:net';
import {delay} from './fixture.mjs';

// A real pinned PocketBase executable authors this fictional backup.
export async function pocketbaseSource(demo){
 const root=join(demo.root,'pocketbase-source');mkdirSync(root);
 const executable=process.env.MIRABASE_DEMO_POCKETBASE||resolve('../_build/demo-pocketbase.exe'),dir=join(root,'data');
 const password=randomBytes(20).toString('hex')+'A1!',email='source@mirabase.example';
 const initialized=spawnSync(executable,['superuser','upsert',email,password,`--dir=${dir}`],{cwd:root,windowsHide:true,encoding:'utf8'});
 if(initialized.status!==0)throw Error('Could not initialize the pinned PocketBase source.');
 const port=await new Promise((ok,no)=>{const s=net.createServer();s.on('error',no);s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>ok(p));});});
 const log=openSync(join(root,'server.log'),'w');const child=spawn(executable,['serve',`--http=127.0.0.1:${port}`,`--dir=${dir}`],{cwd:root,windowsHide:true,stdio:['ignore',log,log]});
 let token;
 async function request(path,body){const r=await fetch(`http://127.0.0.1:${port}`+path,{method:'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:token}:{})},body:JSON.stringify(body)});const raw=await r.text();if(!r.ok)throw Error(`PocketBase source: ${r.status} ${raw}`);return raw?JSON.parse(raw):null;}
 try{
  let healthy=false;for(let i=0;i<100;i++){try{if((await fetch(`http://127.0.0.1:${port}/api/health`)).ok){healthy=true;break;}}catch{}await delay(100);}if(!healthy)throw Error('PocketBase source did not start.');
  token=(await request('/api/collections/_superusers/auth-with-password',{identity:email,password})).token;
  await request('/api/collections',{name:'legacy_notes',type:'base',fields:[{name:'title',type:'text',required:true}]});
  for(const title of ['Customer launch notes','Product research','Team handbook'])await request('/api/collections/legacy_notes/records',{title});
  await request('/api/backups',{name:'pocketbase-source.zip'});
  return join(dir,'backups','pocketbase-source.zip');
 }finally{child.kill();await Promise.race([new Promise(r=>child.once('exit',r)),delay(5000)]);}
}
