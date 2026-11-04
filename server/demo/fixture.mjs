import {spawn, spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,cpSync,writeFileSync,openSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import {randomBytes} from 'node:crypto';
import net from 'node:net';

export const sourceRoot=resolve(dirname(fileURLToPath(import.meta.url)),'..');
export const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function freePort(){return await new Promise((ok,no)=>{const s=net.createServer();s.once('error',no);s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>ok(p));});});}

export async function startDemo({artifact=join(sourceRoot,'dist/mirabase-windows-amd64.exe'),outputRoot}={}){
 const root=mkdtempSync(join(outputRoot||tmpdir(),'mirabase-film-'));
 const publicDir=join(root,'public');mkdirSync(publicDir);
 cpSync(join(sourceRoot,'demo/workbench'),publicDir,{recursive:true});
 cpSync(join(sourceRoot,'sdk/js/dist'),join(publicDir,'sdk'),{recursive:true});
 const password=randomBytes(24).toString('hex')+'A1!';
 const email='operator@mirabase.example';
 const port=await freePort(),pgPort=await freePort(),url=`http://127.0.0.1:${port}`;
 const tls=join(root,'tls');
 const certgen=spawnSync(process.env.MIRABASE_DEMO_GO||'C:/Users/lakesbim/tools/go1.27/go/bin/go.exe',['run','./demo/certgen',tls],{cwd:sourceRoot,windowsHide:true,encoding:'utf8'});
 if(certgen.status!==0)throw Error('Local demo TLS generation failed: '+certgen.stderr);
 const flags=[`--dir=${join(root,'data')}`,`--migrationsDir=${join(root,'migrations')}`,`--hooksDir=${join(root,'hooks')}`];
 const init=spawnSync(artifact,['superuser','upsert',email,password,...flags],{cwd:root,windowsHide:true,encoding:'utf8'});
 if(init.status!==0)throw Error('Demo provisioning failed; no existing data was used.');
 const log=openSync(join(root,'server.log'),'w');
 const proc=spawn(artifact,['serve',`--http=127.0.0.1:${port}`,`--publicDir=${publicDir}`,...flags],{cwd:root,windowsHide:true,stdio:['ignore',log,log],env:{...process.env,MIRABASE_POSTGRES_ENABLED:'1',MIRABASE_POSTGRES_LISTEN:`127.0.0.1:${pgPort}`,MIRABASE_POSTGRES_TLS_CERT:join(tls,'cert.pem'),MIRABASE_POSTGRES_TLS_KEY:join(tls,'key.pem')}});
 const history=[];let adminToken;const diagnostics={root};
 const stop=async()=>{if(proc.exitCode===null){proc.kill();await Promise.race([new Promise(r=>proc.once('exit',r)),delay(10000)]);}};
 const call=async(path,method='GET',body,token=adminToken,expected)=>{
  const headers={'Content-Type':'application/json'};if(token)headers.Authorization=token;
  const response=await fetch(url+path,{method,headers,...(body===undefined?{}:{body:JSON.stringify(body)})});
  const raw=await response.text();let data;try{data=raw?JSON.parse(raw):null;}catch{data=raw;}
  history.push({path,method,status:response.status});
  if(expected!==undefined?response.status!==expected:!response.ok)throw Error(`${method} ${path}: ${response.status} ${raw.slice(0,500)}`);
  return data;
 };
 try{
  let ready=false;
  for(let i=0;i<90;i++){try{if((await fetch(url+'/api/health')).ok){ready=true;break;}}catch{}if(proc.exitCode!==null)break;await delay(250);}
  if(!ready)throw Error('Demo server did not become healthy.');
  adminToken=(await call('/api/collections/_superusers/auth-with-password','POST',{identity:email,password},null)).token;
  const members=await call('/api/collections','POST',{name:'members',type:'auth',fields:[{name:'name',type:'text',required:true},{name:'role',type:'select',values:['Owner','Member','Agent'],maxSelect:1}],passwordAuth:{enabled:true,identityFields:['email']},listRule:'id = @request.auth.id',viewRule:'id = @request.auth.id',updateRule:'id = @request.auth.id'});
  const people={};const memberPassword=randomBytes(18).toString('hex')+'A1!';
  for(const [key,name,role]of[['alice','Alice Morgan','Owner'],['bob','Bob Chen','Member'],['agent','Research Agent','Agent']]){
   people[key]=await call('/api/collections/members/records','POST',{email:`${key}@mirabase.example`,password:memberPassword,passwordConfirm:memberPassword,name,role,verified:true});
  }
  const docs=await call('/api/collections','POST',{name:'docs',type:'base',fields:[{name:'title',type:'text',required:true},{name:'body',type:'text'},{name:'embedding',type:'text'}],listRule:'',viewRule:'',createRule:'@request.auth.id != ""',updateRule:'@request.auth.id != ""',deleteRule:'@request.auth.id != ""'});
  diagnostics.docs=docs;
  const projects=await call('/api/collections','POST',{name:'projects',type:'base',fields:[{name:'name',type:'text',required:true},{name:'status',type:'select',values:['Planning','In progress','Complete'],maxSelect:1},{name:'budget',type:'number',min:0},{name:'owner',type:'relation',collectionId:members.id,maxSelect:1},{name:'notes',type:'editor'},{name:'files',type:'file',maxSelect:5,maxSize:5242880}],listRule:'@request.auth.id != ""',viewRule:'@request.auth.id != ""',createRule:'@request.auth.id != ""',updateRule:'@request.auth.id != ""',deleteRule:'@request.auth.id != ""'});
  const catalogue=await call('/api/collections','POST',{name:'catalogue',type:'base',fields:[{name:'label',type:'text',required:true},{name:'price',type:'number'}],listRule:'',viewRule:''});
  for(const row of[{name:'Customer portal',status:'In progress',budget:48000,owner:people.alice.id,notes:'<p>A frontend connected to one MiraBase backend.</p>'},{name:'Agent research hub',status:'Planning',budget:16000,owner:people.bob.id},{name:'Mobile field workspace',status:'Complete',budget:24000,owner:people.alice.id}])await call('/api/collections/projects/records','POST',row);
  for(const row of[{label:'Starter workspace',price:19},{label:'Team workspace',price:49}])await call('/api/collections/catalogue/records','POST',row);
  const template=await call('/api/mirabase/v1/templates','POST',{templateId:'demo-workspace',collectionNames:['docs'],expectedSchemaRevision:'1'});
  diagnostics.template=template;
  const databases={};
  for(const[key,kind,owner,displayName]of[['alice','user',people.alice,'Alice · private workspace'],['bob','user',people.bob,'Bob · private workspace'],['team','tenant',people.alice,'Northstar · team workspace'],['agent','agent',people.agent,'Research · agent workspace']]){
   const job=await call('/api/mirabase/v1/databases','POST',{kind,displayName,profile:'wal',owner:{collectionId:members.id,recordId:owner.id},templateId:'demo-workspace'});
   if(job.state!=='succeeded')throw Error('Database provisioning is not complete.');
   const list=await call('/api/mirabase/v1/databases');
   const found=list.items?.find(d=>d.displayName===displayName);
   if(!found)throw Error('Created database was not found in the registry.');
   databases[key]=found;
  }
  const idOf=d=>d.databaseId||d.id;
  const contents=[['hello MiraBase','One backend. Private data on your own server.',[1,0,0]],['Team planning guide','Roadmaps, projects and shared collaboration.',[.8,.2,0]],['Vector search notebook','Embeddings and native database queries.',[0,1,0]],['Offline field notes','Work close to the device, synchronize through the record API.',[.6,.1,.3]]];
  for(const key of Object.keys(databases)){
   const dbid=idOf(databases[key]);
   for(const[title,body,embedding]of contents)await call(`/api/mirabase/v1/db/${dbid}/collections/docs/records`,'POST',{title:key==='bob'?'Bob: '+title:title,body,embedding});
   await call(`/api/mirabase/v1/databases/${dbid}/collections/docs/search-index`,'POST',{indexName:'idx_docs_fts',fields:['title','body'],tokenizer:'default'});
  }
  const config={url,members,docs,projects,catalogue,people,databases,template};
  writeFileSync(join(publicDir,'demo-config.json'),JSON.stringify(config,null,2));
  return {root,url,proc,password,email,memberPassword,config,call,history,stop,adminToken,pgPort,pgCA:readFileSync(join(tls,'cert.pem'),'utf8')};
 }catch(error){console.error(JSON.stringify({fixtureDiagnostics:diagnostics,lastRequests:history.slice(-8)},null,2));await stop();throw error;}
}

if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)&&process.argv.includes('--probe')){
 let demo;
 try{demo=await startDemo();console.log(JSON.stringify({ready:true,url:demo.url,root:demo.root,databases:Object.keys(demo.config.databases),apiCalls:demo.history.length}));}
 finally{if(demo)await demo.stop();}
}
