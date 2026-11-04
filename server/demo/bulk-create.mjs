import {readFileSync,writeFileSync,appendFileSync,mkdirSync,statSync,statfsSync,existsSync} from 'node:fs';
import {join,resolve} from 'node:path';
import os from 'node:os';
const session=JSON.parse(readFileSync(resolve(process.argv[2]),'utf8'));
const target=Number(process.argv[3]||100),concurrency=Number(process.argv[4]||4);
const output=resolve(process.argv[5]||'../_build/bulk-pilot');mkdirSync(output,{recursive:true});
if(!Number.isSafeInteger(target)||target<1||!Number.isInteger(concurrency)||concurrency<1||concurrency>8)throw Error('Invalid workload size.');
let next=0,completed=0,failed=0,stopped=null;const start=Date.now();const samples=[];
const headers={Authorization:session.adminToken,'Content-Type':'application/json'};
async function getCount(){const r=await fetch(session.url+'/api/mirabase/v1/databases?perPage=1',{headers});if(!r.ok)throw Error('Registry count failed '+r.status);return(await r.json()).totalItems;}
const baseline=await getCount();
console.log(`MiraBase real public-API workload | target ${target.toLocaleString()} databases | concurrency ${concurrency}`);
console.log(`Server ${session.url} | one backend PID ${session.pid} | baseline registry count ${baseline}`);
console.log('Every counted database must return a succeeded job, a ready database ID, and pass server integrity verification.');
function guard(){const disk=statfsSync(session.root),freeDisk=disk.bavail*disk.bsize;if(freeDisk<10*1024**3)stopped='10 GiB free-disk floor';if(os.freemem()<3*1024**3)stopped='3 GiB available-memory floor';if(existsSync(join(output,'STOP')))stopped='operator stop';return{freeDiskBytes:freeDisk,availableMemoryBytes:os.freemem()};}
function progress(){const resources=guard(),seconds=(Date.now()-start)/1000;const p={target,completed,failed,elapsedSeconds:seconds,databasesPerSecond:completed/Math.max(seconds,.001),stopped,...resources};writeFileSync(join(output,'progress.json'),JSON.stringify(p,null,2));console.log(`${completed.toLocaleString()} / ${target.toLocaleString()} real databases | ${p.databasesPerSecond.toFixed(1)}/sec | failures ${failed} | ${(resources.freeDiskBytes/1024**3).toFixed(1)} GiB free${stopped?' | STOP: '+stopped:''}`);return p;}
const timer=setInterval(progress,5000);
async function worker(){while(!stopped){const index=next++;if(index>=target)return;guard();if(stopped)return;
 try{const r=await fetch(session.url+'/api/mirabase/v1/databases',{method:'POST',headers,body:JSON.stringify({kind:'manual',profile:'wal',displayName:`screen-load-${Date.now()}-${index}`}),signal:AbortSignal.timeout(45000)});const raw=await r.text();if(r.status!==202)throw Error(`HTTP ${r.status}: ${raw.slice(0,300)}`);const job=JSON.parse(raw),result=JSON.parse(job.resultRef||'{}');if(job.state!=='succeeded'||result.state!=='ready'||!result.databaseId)throw Error('Creation was not verified ready.');
  const dir=join(session.root,'data','databases',result.databaseId);const manifest=JSON.parse(readFileSync(join(dir,'manifest.json'),'utf8'));if(manifest.databaseId!==result.databaseId)throw Error('Physical manifest mismatch.');const file=statSync(join(dir,'data.db'));completed++;
  if(samples.length<12||index%1000===0)samples.push({id:result.databaseId,engineFileBytes:file.size,manifestBytes:statSync(join(dir,'manifest.json')).size});
  appendFileSync(join(output,'created.ndjson'),JSON.stringify({index,id:result.databaseId,job:job.id})+'\n');
 }catch(error){failed++;stopped=error.message;appendFileSync(join(output,'errors.ndjson'),JSON.stringify({index,error:error.message})+'\n');}
 }}
try{await Promise.all(Array.from({length:concurrency},worker));}
finally{clearInterval(timer);const final=progress();final.baselineRegistryCount=baseline;final.finalRegistryCount=await getCount();final.registryIncrease=final.finalRegistryCount-baseline;final.samples=samples;final.completedAsRequested=completed===target&&!failed&&!stopped;writeFileSync(join(output,'result.json'),JSON.stringify(final,null,2));console.log(JSON.stringify(final,null,2));if(!final.completedAsRequested||final.registryIncrease!==completed)process.exitCode=1;}
