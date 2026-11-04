const config=await fetch('/demo-config.json').then(r=>r.json());
const $=s=>document.querySelector(s), esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const db=k=>config.databases[k].databaseId||config.databases[k].id;
const base=k=>`/api/mirabase/v1/db/${db(k)}/collections/docs/records`;
const admin=k=>`/api/mirabase/v1/databases/${db(k)}`;
const state={token:'',tokens:{},requests:[],evidence:[],streams:[],active:'overview'};
const clean=(value)=>JSON.parse(JSON.stringify(value, (key,v)=>/token|password|secret|cursor|credential|dsn/i.test(key)?'[redacted]':v));
function show(value){$('#response').textContent=JSON.stringify(clean(value),null,2);}
function notice(text){$('#notice').textContent=text;$('#notice').classList.add('visible');setTimeout(()=>$('#notice').classList.remove('visible'),3800);}
async function api(path,method='GET',body,token=state.token,expected){
 const headers={Authorization:token};if(!(body instanceof FormData))headers['Content-Type']='application/json';
 const start=performance.now();const response=await fetch(path,{method,headers,...(body===undefined?{}:{body:body instanceof FormData?body:JSON.stringify(body)})});
 const raw=await response.text();let value;try{value=raw?JSON.parse(raw):null;}catch{value=raw;}
 const request={path,method,status:response.status,milliseconds:Math.round(performance.now()-start),response:clean(value)};
 state.evidence.push(request);state.requests.unshift(request);state.requests.length=Math.min(12,state.requests.length);
 $('#requests').innerHTML=state.requests.map(r=>`<div class="request-row"><b>${r.method}</b><span>${esc(r.path)}</span><em class="${r.status>=400?'error':''}">${r.status}</em></div>`).join('');show(value);
 if(expected===undefined?!response.ok:response.status!==expected)throw Error(`${method} ${path}: ${response.status} ${typeof value==='object'?value.message:raw}`);
 return value;
}
const card=(label,value,text)=>`<div class="card"><span class="overline">${label}</span><strong>${value}</strong><p>${text}</p></div>`;
const table=(rows,cols)=>`<table class="data-table"><thead><tr>${cols.map(([k,label])=>`<th>${label||k}</th>`).join('')}</tr></thead><tbody>${rows.map(row=>`<tr>${cols.map(([k])=>`<td>${esc(typeof row[k]==='object'?JSON.stringify(row[k]):row[k])}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
const button=(id,label,secondary=false)=>`<button class="action ${secondary?'secondary':''}" data-action="${id}">${label}</button>`;
function panel(title,description,buttons,body=''){return `<div class="panel"><h2>${title}</h2><p>${description}</p><div class="toolbar">${buttons}</div><div id="output">${body}</div></div>`;}
function rows(value){return value.items||value.records||value.rows||value.results||[];}
const sections=[
 ['overview','01 · Your backend','One backend. A private database for every user, tenant, and agent.','This is a connected example frontend. The admin link opens the actual MiraBase dashboard.'],
 ['records','02 · Collections & records','Your frontend, connected.','Collections, typed fields, relations, validation, filters, files and batch requests remain available through the inherited backend.'],
 ['auth','03 · Identity & rules','Identity belongs to the backend.','Real account sign-in and collection rules. The operator credential stays in memory and is redacted from the recording.'],
 ['files','04 · Files & realtime','Upload a file. See a change arrive.','File storage and record subscriptions use the real backend. No responses are intercepted or simulated.'],
 ['databases','05 · Private databases','Many databases. One server process.','User databases isolate individuals. Tenant databases are shared by a team. Agent databases have their own owner binding.'],
 ['access','06 · Grants & isolation','Access follows explicit authority.','Bob starts without access to Alice’s database. A read grant permits reads; revocation removes it.'],
 ['search','07 · Search','Four ways to find an answer.','Run native full-text, vector, hybrid and fuzzy search against a real private database.'],
 ['changes','08 · Durable changes','Committed records leave a durable trail.','Create, update and delete a record, then inspect actual journal entries.'],
 ['sync','09 · Record sync','Bootstrap, push, pull, acknowledge.','This demonstrates MiraBase’s record protocol. It is separate from upstream native database replication.'],
 ['live','10 · Live SQL results','Subscribe to a query result.','Publish a deterministic plan, open its stream, then observe a real committed change.'],
 ['schema','11 · Schemas & templates','Schema changes are explicit plans.','Inspect the published template, preview a target change, and apply its verified plan.'],
 ['references','12 · Shared reference data','Keep a local copy of shared information.','A field-scoped catalogue policy refreshes real copies inside private databases.'],
 ['reports','13 · Reporting','Aggregate across matching databases.','Reports execute real SQL over the databases selected by a source template.'],
 ['workspace','14 · Agent workspaces','An agent can keep working state.','Write a file, snapshot it, change it, then restore its historical bytes. This is the workspace adapter, not a claim of full upstream AgentFS integration.'],
 ['operations','15 · Operations','Inspect, maintain and back up.','Diagnostics, durable jobs, maintenance controls and archives are real server operations.'],
 ['sdk','16 · Client SDK','Scope the client to a database.','The bundled JavaScript client can hold separate immutable views without a shared current-database switch.'],
 ['pg','17 · PostgreSQL client','Your existing SQL client, connected.','A real node-postgres process connects to this backend through its TLS/SCRAM gateway. This is scoped SQL access, separate from record validation.'],
];
const actions={};let cleanup;
async function render(key){
 if(cleanup){cleanup();cleanup=null;}
 state.active=key;const section=sections.find(s=>s[0]===key);$('#chapter-kicker').textContent=section[1];$('#chapter-title').textContent=section[2];$('#chapter-description').textContent=section[3];
 document.querySelectorAll('nav button').forEach(b=>b.classList.toggle('active',b.dataset.section===key));$('#content').innerHTML='';$('#response').textContent='Select an action to inspect its real response.';
 const content=$('#content');
 switch(key){
 case 'overview': content.innerHTML=`<div class="cards">${card('Backend','One process','HTTP, the admin dashboard and shared services.')}${card('Engine','Turso','In-process native engine; no database sidecar service.')}${card('Layouts','User · tenant · agent','Private files managed by the same running server.')}${card('Deployment','Your server','Application data lives outside the executable.')}</div>`+panel('The application is running','Explore the actual server, not a slide deck.',button('health','Check server health')+button('overview-dbs','Inspect database registry',true));break;
 case 'records':content.innerHTML=panel('Projects','Use the ordinary record API with typed fields and an owner relation.',button('projects','Load projects')+button('create-project','Create project',true)+button('invalid-project','Show validation',true)+button('filter-project','Filter & expand owner',true)+button('batch','Atomic batch',true));break;
 case 'auth':content.innerHTML=panel('Alice’s account','Password authentication and collection access rules are evaluated by the server.',button('sign-in','Sign in as Alice')+button('own-profile','Read own profile',true)+button('wrong-profile','Try Bob’s protected profile',true)+button('auth-config','Inspect auth capabilities',true));break;
 case 'files':content.innerHTML=panel('Files and events','Attach a real text file to a project. Open a record subscription, then commit a change.',button('upload-file','Upload attachment')+button('download-file','Read uploaded bytes',true)+button('realtime','Subscribe to projects',true)+button('realtime-change','Create realtime event',true),'<div id="events" class="event-list"></div>');break;
 case 'databases':content.innerHTML=panel('Managed database registry','The four demo databases have different owner bindings and different files.',button('list-dbs','Refresh registry')+button('alice-data','Open Alice’s database',true)+button('bob-data','Open Bob’s database',true)+button('team-data','Open tenant database',true)+button('agent-data','Open agent database',true));break;
 case 'access':content.innerHTML=panel('Cross-database access','Watch the server refuse, allow, then refuse the same user.',button('access-denied','Bob tries Alice’s database')+button('grant','Grant Bob read access',true)+button('granted-read','Read with grant',true)+button('revoke','Revoke & retry',true));break;
 case 'search':content.innerHTML=panel('Search Alice’s private workspace','Each button sends its own real search request.',button('text-search','Full-text search')+button('vector-search','Vector similarity',true)+button('hybrid-search','Hybrid ranking',true)+button('fuzzy-search','Fuzzy matching',true));break;
 case 'changes':content.innerHTML=panel('Durable journal','Record mutations capture committed changes.',button('journal-create','Create a note')+button('journal-update','Update it',true)+button('journal-delete','Delete it',true)+button('journal-read','Read change journal',true));break;
 case 'sync':content.innerHTML=panel('Record synchronization','An operator opens an immutable bootstrap page. A real mutation group is then pushed and pulled.',button('bootstrap','Create bootstrap')+button('bootstrap-page','Read snapshot page',true)+button('push','Push a mutation group',true)+button('pull','Pull changes',true)+button('ack','Acknowledge cursor',true));break;
 case 'live':content.innerHTML=panel('Live query plan','Snapshot and delta frames come from an actual streaming response.',button('publish-live','Publish query plan')+button('live-stream','Open result stream',true)+button('live-mutate','Change the result',true)+button('live-cancel','Cancel subscription',true),'<div id="events" class="event-list"></div>');break;
 case 'schema':content.innerHTML=panel('Target schema','A schema plan binds its target, generation and expected revision.',button('get-template','Inspect template')+button('schema-plan','Plan a new table',true)+button('schema-apply','Apply verified plan',true));break;
 case 'references':content.innerHTML=panel('Shared catalogue','Select the label field for user database copies.',button('define-reference','Define reference policy')+button('refresh-reference','Refresh real copies',true)+button('read-reference','Read Alice’s copy',true)+button('reference-status','Inspect freshness',true));break;
 case 'reports':content.innerHTML=panel('Workspace report','Count records across databases using the workspace template.',button('define-report','Define aggregate')+button('rebuild-report','Rebuild report',true)+button('get-report','Inspect checkpoint',true));break;
 case 'workspace':content.innerHTML=panel('Research agent’s working files','The snapshot must restore old content after the current file changes.',button('workspace-create','Create workspace')+button('workspace-write','Write research note',true)+button('workspace-snapshot','Snapshot',true)+button('workspace-change','Replace content',true)+button('workspace-restore','Restore historical bytes',true),'<div id="file" class="file-preview">No file has been read yet.</div>');break;
 case 'operations':content.innerHTML=panel('Operate the server','Use diagnostics and inspect the actual completed operations.',button('diagnostics','Database diagnostics')+button('jobs','Inspect durable jobs',true)+button('export','Export private database',true)+button('backup','Create server backup',true)+button('backups','List backup archives',true));break;
 case 'sdk':content.innerHTML=panel('Bundled JavaScript client','The example loads the SDK shipped in this release source.',button('sdk-parallel','Read Alice and Bob concurrently')+button('sdk-path-refusal','Reject credential path escape',true));break;
 case 'pg':content.innerHTML=panel('External client · node-postgres','The recording controller starts a real external client. TLS certificate verification stays enabled.',button('pg-client','Run PostgreSQL client query'));break;
 }
 $('#connection-status').textContent='Connected · '+new URL(config.url).host;
}
function output(value,columns){$('#output').innerHTML=table(rows(value),columns);}
function event(type,value){const el=$('#events');if(el)el.insertAdjacentHTML('afterbegin',`<div class="event ${esc(type)}"><b>${esc(type.toUpperCase())}</b>${esc(JSON.stringify(clean(value)))}</div>`);}
async function stream(path){const controller=new AbortController();state.streams.push(controller);cleanup=()=>controller.abort();const response=await fetch(path,{headers:{Authorization:state.token},signal:controller.signal});if(!response.ok)throw Error('Stream refused: '+response.status);state.evidence.push({path,method:'GET SSE',status:response.status});const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='';(async()=>{try{while(true){const {value,done}=await reader.read();if(done)break;buffer+=decoder.decode(value,{stream:true});buffer=buffer.replace(/\r\n/g,'\n');let index;while((index=buffer.indexOf('\n\n'))>=0){const block=buffer.slice(0,index);buffer=buffer.slice(index+2);let type='message',data='';for(const line of block.split('\n')){if(line.startsWith('event:'))type=line.slice(6).trim();if(line.startsWith('data:'))data+=line.slice(5).trim();}if(data){let parsed;try{parsed=JSON.parse(data);}catch{parsed=data;}event(type,parsed);state.evidence.push({stream:path,type,data:clean(parsed)});}}}}catch(e){if(e.name!=='AbortError')notice(e.message);}})();return controller;}
actions.health=async()=>api('/api/health');
actions['overview-dbs']=actions['list-dbs']=async()=>{const r=await api('/api/mirabase/v1/databases');output(r,[['displayName','Database'],['kind','Layout'],['state','State'],['schemaRevision','Schema']]);};
actions.projects=async()=>output(await api('/api/collections/projects/records?sort=name'),[['name','Project'],['status','Status'],['budget','Budget'],['owner','Owner']]);
actions['create-project']=async()=>{await api('/api/collections/projects/records','POST',{name:'Launch workspace',status:'Planning',budget:12500,owner:config.people.alice.id});await actions.projects();notice('Project persisted in the backend.');};
actions['invalid-project']=async()=>{await api('/api/collections/projects/records','POST',{name:'',budget:-10},state.token,400);notice('Server rejected the missing name and negative budget.');};
actions['filter-project']=async()=>{const r=await api('/api/collections/projects/records?filter='+encodeURIComponent('budget >= 20000')+'&expand=owner&sort=-budget');output(r,[['name','Matching project'],['budget','Budget'],['owner','Owner ID']]);};
actions.batch=async()=>{await api('/api/settings','PATCH',{batch:{enabled:true,maxRequests:20,timeout:3,maxBodySize:1048576}});const r=await api('/api/batch','POST',{requests:[{method:'POST',url:'/api/collections/projects/records',body:{name:'Batch · design',status:'Planning',budget:5000}},{method:'POST',url:'/api/collections/projects/records',body:{name:'Batch · engineering',status:'In progress',budget:9000}}]});$('#output').innerHTML=table(r,[['status','Per-request status'],['body','Saved record']]);};
actions['sign-in']=async()=>{const result=await api('/api/collections/members/auth-with-password','POST',{identity:config.people.alice.email,password:state.memberPassword},'');state.tokens.alice=result.token;notice('Alice signed in through the actual password authentication endpoint.');};
actions['own-profile']=async()=>{const r=await api('/api/collections/members/records/'+config.people.alice.id,'GET',undefined,state.tokens.alice);$('#output').innerHTML=table([r],[['name','Name'],['role','Role'],['id','Record ID']]);};
actions['wrong-profile']=async()=>{await api('/api/collections/members/records/'+config.people.bob.id,'GET',undefined,state.tokens.alice,404);notice('Alice cannot read Bob’s profile.');};
actions['auth-config']=async()=>{const r=await api('/api/collections/members/auth-methods');show(r);};
actions['upload-file']=async()=>{const form=new FormData();form.append('name','Launch checklist');form.append('status','Planning');form.append('files',new File(['MiraBase launch checklist\n1. Connect the frontend\n2. Keep private data private\n3. Back up the application\n'],'launch-checklist.txt',{type:'text/plain'}));state.file=await api('/api/collections/projects/records','POST',form);notice('File uploaded with its project record.');};
actions['download-file']=async()=>{if(!state.file)throw Error('Upload the file first.');const r=await api(`/api/files/${config.projects.id}/${state.file.id}/${state.file.files[0]}`);$('#output').innerHTML=`<pre class="file-preview">${esc(r)}</pre>`;};
actions.realtime=async()=>{const controller=new AbortController();state.streams.push(controller);cleanup=()=>controller.abort();const response=await fetch('/api/realtime',{headers:{Authorization:state.token},signal:controller.signal});const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='';(async()=>{try{while(true){const{value,done}=await reader.read();if(done)break;buffer+=decoder.decode(value,{stream:true});buffer=buffer.replace(/\r\n/g,'\n');let index;while((index=buffer.indexOf('\n\n'))>=0){const block=buffer.slice(0,index);buffer=buffer.slice(index+2);const match=block.match(/data:\s*(.+)/);if(!match)continue;const data=JSON.parse(match[1]);if(data.clientId){await api('/api/realtime','POST',{clientId:data.clientId,subscriptions:['projects/*']});event('connected',{clientId:'[redacted]',subscription:'projects/*'});}else{event(data.action||'event',data);state.evidence.push({stream:'realtime',data:clean(data)});}}}}catch(e){if(e.name!=='AbortError')notice(e.message);}})();notice('Real record subscription opened.');};
actions['realtime-change']=async()=>{await api('/api/collections/projects/records','POST',{name:'Realtime · created now',status:'In progress',budget:8000});};
for(const key of ['alice','bob','team','agent'])actions[key+'-data']=async()=>output(await api(base(key)),[['title','Document'],['body','Content'],['id','Record ID']]);
actions['access-denied']=async()=>{await api(base('alice'),'GET',undefined,state.tokens.bob,403);notice('Different owner: access refused.');};
actions.grant=async()=>{state.grant=await api(admin('alice')+'/grants','PUT',{subject:{collectionId:config.members.id,recordId:config.people.bob.id},scopes:['record.read'],expectedEpoch:1});notice('Bob now has an explicit read grant.');};
actions['granted-read']=async()=>{output(await api(base('alice'),'GET',undefined,state.tokens.bob),[['title','Document'],['id','Record ID']]);};
actions.revoke=async()=>{await api(admin('alice')+'/grants/'+state.grant.id,'DELETE');await api(base('alice'),'GET',undefined,state.tokens.bob,403);notice('Revocation took effect on the next request.');};
const search=async body=>{const r=await api(admin('alice')+'/collections/docs/search','POST',{...body,limit:5});const list=rows(r);$('#output').innerHTML=list.length?`<div class="search-grid">${list.map(item=>{const row=item.record||item;return `<div class="result"><h3>${esc(row.title||row.id)}</h3><p>${esc(row.body||'')}</p><small>${esc(JSON.stringify(item.scores||item.score||item.distance||''))}</small></div>`;}).join('')}</div>`:`<pre>${esc(JSON.stringify(r,null,2))}</pre>`;};
actions['text-search']=()=>search({mode:'text',text:{fields:['title','body'],query:'workspace'}});
actions['vector-search']=()=>search({mode:'vector',vector:{field:'embedding',query:[1,0,0],metric:'cosine'}});
actions['hybrid-search']=()=>search({mode:'hybrid',text:{fields:['title','body'],query:'workspace'},vector:{field:'embedding',query:[1,0,0],metric:'cosine'},fusion:{kind:'rrf',k:60}});
actions['fuzzy-search']=()=>search({mode:'fuzzy',field:'title',query:'hello Mirabase',algorithm:'levenshtein',maximumDistance:3});
actions['journal-create']=async()=>{state.journal=await api(base('alice'),'POST',{title:'Journal demonstration',body:'First version'});};
actions['journal-update']=async()=>api(base('alice')+'/'+state.journal.id,'PATCH',{body:'Second version, committed'});
actions['journal-delete']=async()=>api(base('alice')+'/'+state.journal.id,'DELETE');
actions['journal-read']=async()=>{const r=await api(admin('alice')+'/changes?limit=30');show(r);$('#output').innerHTML='<div class="summary"><i></i>Inspect the real journal response below, including commit sequence and action.</div>';};
actions.bootstrap=async()=>{state.bootstrap=await api(admin('alice')+'/sync/bootstrap','POST',{collections:['docs'],clientInstanceId:'film-device-1'});};
actions['bootstrap-page']=async()=>api(admin('alice')+`/sync/bootstrap/${state.bootstrap.sessionId||state.bootstrap.id}/pages/0`);
actions.push=async()=>{const d=await api(admin('alice'));state.pushed=await api(admin('alice')+'/sync/push','POST',{clientInstanceId:'film-device-1',schemaRevision:d.schemaRevision,generation:d.generation,policyEpoch:d.policyEpoch,groups:[{groupId:'film-group-1',mutations:[{mutationId:'film-mutation-1',collection:'docs',recordId:'filmoffline0001',action:'create',data:{title:'Note from a client mutation',body:'Submitted through record sync'}}]}]});};
actions.pull=async()=>{state.pull=await api(admin('alice')+'/sync/pull?limit=100');};
actions.ack=async()=>api(admin('alice')+'/sync/ack','POST',{clientInstanceId:'film-device-1',cursor:state.pull.cursor||state.pull.nextCursor||state.pull.toCursor});
actions['publish-live']=async()=>{state.plan=await api('/api/mirabase/v1/live-query-plans','POST',{sql:'SELECT id, title, body FROM docs ORDER BY title',parameters:[],resultKey:'id',dependencies:['docs'],maxRows:100,maxBytes:65536});await api(`/api/mirabase/v1/live-query-plans/${state.plan.id}/revisions/${state.plan.revision}/publish`,'POST');};
actions['live-stream']=async()=>{const d=await api(admin('alice'));state.live=await api(admin('alice')+'/live-queries','POST',{queryId:state.plan.id,parameters:[],schemaRevision:d.schemaRevision});state.liveController=await stream(state.live.streamUrl+'?cursor='+encodeURIComponent(state.live.cursor));};
actions['live-mutate']=async()=>{await api(base('alice'),'POST',{title:'Live · newly committed result',body:'This row arrived after the stream opened.'});};
actions['live-cancel']=async()=>{const r=await api(admin('alice')+`/live-queries/${state.plan.id}`,'DELETE',{id:state.live.id});state.liveController.abort();event('closed',r);};
actions['get-template']=async()=>api('/api/mirabase/v1/templates/demo-workspace');
actions['schema-plan']=async()=>{const d=await api(admin('agent'));state.schema=await api(admin('agent')+'/schema/plans','POST',{expectedSchemaRevision:d.schemaRevision,operations:[{kind:'collection.create',objectId:'observations',definition:{id:'observations',name:'observations',type:'base',fields:[{id:'note_field',name:'note',type:'text'}]}}]});if(state.schema.blockers.length)throw Error('Schema plan contains unresolved blockers.');};
actions['schema-apply']=async()=>api(admin('agent')+`/schema/plans/${state.schema.id||state.schema.planId}/apply`,'POST',{planHash:state.schema.planHash,expectedSchemaRevision:state.schema.expectedSchemaRevision||state.schema.schemaRevision||1});
actions['define-reference']=async()=>api('/api/mirabase/v1/reference-copies/demo_catalogue','PUT',{revision:1,sourceCollectionId:config.catalogue.id,fieldIds:['label'],filter:'',targets:{kinds:['user']},deleteBehavior:'remove-copy',maximumLagSeconds:60});
actions['refresh-reference']=async()=>api('/api/mirabase/v1/reference-copies/demo_catalogue/refresh','POST',{revision:1});
actions['read-reference']=async()=>{const r=await api(admin('alice')+'/reference-copies/demo_catalogue/records');show(r);};
actions['reference-status']=async()=>api('/api/mirabase/v1/reference-copies/demo_catalogue');
actions['define-report']=async()=>api('/api/mirabase/v1/reports/demo_summary','PUT',{revision:1,sourceTemplateIds:['demo-workspace'],projectedFieldIds:['title'],aggregateSql:'SELECT COUNT(*) AS n FROM docs',targetCollectionId:config.docs.id,freshnessSeconds:60,readerRule:''});
actions['rebuild-report']=async()=>api('/api/mirabase/v1/reports/demo_summary/rebuild','POST');
actions['get-report']=async()=>api('/api/mirabase/v1/reports/demo_summary');
const wp=()=>`/api/mirabase/v1/agentfs/workspaces/${state.workspace.id}`;
const readFile=async()=>{const r=await api(wp()+'/files?path=research/notes.txt');$('#file').textContent=typeof r==='string'?r:r.content;return r;};
actions['workspace-create']=async()=>{state.workspace=await api('/api/mirabase/v1/agentfs/workspaces','POST',{ownerCollectionId:config.members.id,ownerRecordId:config.people.agent.id,displayName:'Research workspace'});};
actions['workspace-write']=async()=>{await api(wp()+'/files','PUT',{path:'research/notes.txt',content:'Research notebook — version one\nPrivate customer data belongs in a private database.\nNext: validate the search results.'});await readFile();};
actions['workspace-snapshot']=async()=>{state.snapshot=await api(wp()+'/snapshots','POST');notice('Snapshot stored with the historical file bytes.');};
actions['workspace-change']=async()=>{await api(wp()+'/files','PUT',{path:'research/notes.txt',content:'Version two: this replaces the original note.\nWe will restore version one from the snapshot.'});await readFile();};
actions['workspace-restore']=async()=>{await api(wp()+`/snapshots/${state.snapshot.id||state.snapshot.snapshotId}/restore`,'POST');await readFile();notice('Original bytes restored from the snapshot.');};
actions.diagnostics=async()=>api(admin('alice')+'/diagnostics');
actions.jobs=async()=>{const r=await api('/api/mirabase/v1/jobs');output(r,[['type','Operation'],['state','State'],['id','Job ID']]);};
actions.export=async()=>api(admin('alice')+'/exports','POST',{format:'portable-v1',includeFiles:true});
actions.backup=async()=>api('/api/backups','POST',{name:'mirabase-demo.zip'});
actions.backups=async()=>{const r=await api('/api/backups');show(r);$('#output').innerHTML=table(r,[['key','Archive'],['size','Bytes'],['modified','Modified']]);};
actions['sdk-parallel']=async()=>{const {MirabaseClient}=await import('/sdk/index.js');const client=new MirabaseClient(location.origin,()=>state.token);state.sdk=client;const alice=client.withContext({databaseId:db('alice')}),bob=client.withContext({databaseId:db('bob')});const result=await Promise.all([alice.listRecords('docs'),bob.listRecords('docs')]);show({alice:result[0],bob:result[1]});$('#output').innerHTML='<div class="summary"><i></i>Both requests completed through separate immutable SDK views.</div>';state.evidence.push({sdk:'JavaScript',operation:'concurrent scoped reads',result:clean(result)});};
actions['sdk-path-refusal']=async()=>{try{await state.sdk.request('//elsewhere.example/steal');throw Error('Escape was incorrectly accepted.');}catch(e){if(!e.message.includes('single slash'))throw e;show({refused:true,message:e.message});state.evidence.push({sdk:'JavaScript',operation:'credential escape negative',refused:true});}};
actions['pg-client']=async()=>{if(!window.runExternalPG)throw Error('The external-client recording controller is not attached.');const result=await window.runExternalPG();show(result);$('#output').innerHTML=`<div class="summary"><i></i>${esc(result.client)} · ${esc(result.connection.protocol)} · certificate verified: ${result.connection.tlsAuthorized}</div><pre>${esc(result.sql)}</pre>`+table(result.rows,[['title','SQL result'],['body','Content']]);state.evidence.push({externalClient:'node-postgres',result});};
$('#navigation').innerHTML=sections.map(([key,label])=>`<button data-section="${key}">${label}</button>`).join('');
$('#navigation').onclick=e=>{const b=e.target.closest('[data-section]');if(b)render(b.dataset.section);};
document.addEventListener('click',async e=>{const b=e.target.closest('[data-action]');if(!b)return;b.disabled=true;try{await actions[b.dataset.action]();b.dataset.result='passed';}catch(error){b.dataset.result='failed';state.evidence.push({action:b.dataset.action,error:error.message});notice(error.message);console.error(error);}finally{b.disabled=false;}});
window.film={state,render,api,sections,async authenticate({email,password,memberPassword}){state.memberPassword=memberPassword;state.token=(await api('/api/collections/_superusers/auth-with-password','POST',{identity:email,password},'')).token;for(const key of ['alice','bob'])state.tokens[key]=(await api('/api/collections/members/auth-with-password','POST',{identity:config.people[key].email,password:memberPassword},'')).token;}};
$('#server-url').textContent=new URL(config.url).host;
await render('overview');



