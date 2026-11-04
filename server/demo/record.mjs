import {startDemo,delay} from './fixture.mjs';
import {story} from './story.mjs';
import {runPostgres} from './external.mjs';
import {pocketbaseSource} from './pocketbase-source.mjs';
import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
const root=resolve('..'),out=join(root,'_build/hackathon-film');mkdirSync(out,{recursive:true});
let playwright;try{playwright=await import('playwright-core');}catch{playwright=await import(pathToFileURL(join(root,'_build/pwtools/node_modules/playwright-core/index.mjs')));}
const {chromium}=playwright;
const frames=process.argv.includes('--resume')?JSON.parse(readFileSync(join(out,'recording.json'),'utf8')).scenes:[];let demo,browser;
const sha=path=>createHash('sha256').update(readFileSync(path)).digest('hex');
const credentials=()=>({email:demo.email,password:demo.password,memberPassword:demo.memberPassword});
async function login(page){await page.goto(demo.url+'/_/');await page.locator('#login_identity').fill(demo.email);await page.locator('#login_pass').fill(demo.password);await page.locator('#login_pass').press('Enter');await page.waitForFunction(()=>!document.querySelector('#login_pass'));await page.getByText('projects',{exact:true}).first().waitFor();}
async function pointer(page){await page.addStyleTag({content:'#film-pointer{position:fixed;left:-30px;top:-30px;width:17px;height:17px;background:#fff;border:2px solid #0b1d39;border-radius:50%;box-shadow:0 0 0 6px #74a6f72c;z-index:2147483647;pointer-events:none;transform:translate(-50%,-50%)}#film-pointer.down{box-shadow:0 0 0 16px #74a6f74c;transition:box-shadow .15s}'});await page.evaluate(()=>{const p=document.createElement('div');p.id='film-pointer';document.body.append(p);document.addEventListener('mousemove',e=>{p.style.left=e.clientX+'px';p.style.top=e.clientY+'px'});document.addEventListener('mousedown',()=>p.classList.add('down'));document.addEventListener('mouseup',()=>p.classList.remove('down'));});}
async function clickAction(page,id){
 const button=page.locator(`[data-action="${id}"]`);await button.scrollIntoViewIfNeeded();await button.click();
 await page.waitForFunction(id=>document.querySelector(`[data-action="${id}"]`)?.dataset.result,id,{timeout:45000});
 if(await button.getAttribute('data-result')!=='passed')throw Error('Recording action failed: '+id+' · '+await page.locator('#notice').textContent());
 if(id==='realtime')await page.waitForFunction(()=>film.state.evidence.some(e=>e.path==='/api/realtime'&&e.method==='POST'));
 if(id==='realtime-change')await page.waitForFunction(()=>film.state.evidence.some(e=>e.stream==='realtime'));
 if(id==='live-mutate')await page.waitForFunction(()=>film.state.evidence.some(e=>e.type==='update'));
}
try{
 demo=await startDemo();const source=await pocketbaseSource(demo);
 browser=await chromium.launch({executablePath:process.env.MIRABASE_DEMO_CHROME||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true,args:['--disable-background-networking']});
 for(const scene of story){
  if(frames.some(frame=>frame.id===scene.id))continue;
  const dir=join(out,scene.id);mkdirSync(dir,{recursive:true});
  const context=await browser.newContext({viewport:{width:1920,height:1080},deviceScaleFactor:1,recordVideo:{dir,size:{width:1920,height:1080}}});
  const beforePage=Date.now(),page=await context.newPage();const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  if(['dashboard','import'].includes(scene.id)){
   await login(page);
   await page.goto(demo.url+(scene.id==='dashboard'?'/_/#/collections?collection=projects':'/_/#/settings/mirabase-import'));
   if(scene.id==='dashboard')await page.getByText('Customer portal',{exact:true}).first().waitFor();
   else {await page.locator('#mirabase_import_source_file').setInputFiles(source);await page.getByRole('button',{name:'Create session and upload'}).waitFor();}
  }else if(['roadmap','outro'].includes(scene.id)){
   await page.goto(demo.url+'/roadmap.html'+(scene.id==='outro'?'#end':''));
  }else{
   await page.exposeFunction('runExternalPG',()=>runPostgres(demo));
   await page.goto(demo.url);await page.waitForFunction(()=>!!window.film);await page.evaluate(c=>film.authenticate(c),credentials());
   await page.evaluate(id=>film.render(id),scene.id);
  }
  await pointer(page);await delay(350);
  const ready=Date.now(),trimStart=(ready-beforePage)/1000;
  console.log('Recording '+scene.id+' · '+scene.seconds+' seconds');
  if(scene.id==='dashboard'){
   // Editorial title overlay over the genuine running dashboard.
   await page.evaluate(()=>{
    const title=document.createElement('div');title.id='film-opening';
    title.innerHTML='<div style="font-size:29px;font-weight:700;margin-bottom:63px">m &nbsp; MiraBase</div><div style="color:#89b3ef;letter-spacing:3px;font-size:18px;margin-bottom:28px">SELF-HOSTED · GO · TURSO</div><div style="font-size:88px;line-height:1.1;font-weight:750;letter-spacing:-3px">One backend.<br><span style="color:#8cbaff">Private databases.</span></div><div style="font-size:29px;color:#c2d4ee;margin-top:31px">For every user, tenant, and agent.</div>';
    title.style.cssText='position:fixed;inset:0;z-index:2147483645;background:rgba(8,18,36,.95);color:white;padding:120px 150px;font-family:Segoe UI,sans-serif;pointer-events:none;transition:opacity .45s';
    document.body.append(title);setTimeout(()=>title.style.opacity='0',2900);setTimeout(()=>title.remove(),3400);
   });
   await delay(2200);await page.getByText('Customer portal',{exact:true}).first().click();await delay(3000);await page.keyboard.press('Escape');
   await page.evaluate(()=>app.modals.openCollectionUpsert(app.store.activeCollection));
  }else if(scene.id==='import'){
   await page.getByRole('button',{name:'Create session and upload'}).click();
   await page.getByRole('button',{name:'Run',exact:true}).nth(0).waitFor();
   const run=page.getByRole('button',{name:'Run',exact:true});
   for(let index=0;index<5;index++){
    await run.nth(index).waitFor();await page.waitForFunction(i=>{const buttons=[...document.querySelectorAll('button')].filter(b=>b.textContent.trim()==='Run');return buttons[i]&&!buttons[i].disabled},index,{timeout:20000});
    const validation=index===4?page.waitForResponse(r=>r.url().endsWith('/validate')&&r.request().method()==='POST'):null;
    await run.nth(index).click();
    if(validation){const response=await validation;const report=await response.json();if(!response.ok()||report.passed!==true||report.unresolvedErrors!==0)throw Error('Actual import validation failed: '+JSON.stringify(report));}
    await delay(130);
   }
   await page.getByText('Validation',{exact:true}).last().waitFor({timeout:20000});
   await page.getByText('Validation',{exact:true}).last().scrollIntoViewIfNeeded();
   if(await page.locator('.alert.danger:visible').count())throw Error('Import UI surfaced a failure.');
  }else if(scene.actions){
   const gap=Math.max(.6,(scene.seconds-1.1)/scene.actions.length);
   for(let index=0;index<scene.actions.length;index++){
    const due=ready+500+index*gap*1000;await delay(Math.max(0,due-Date.now()));
    await clickAction(page,scene.actions[index]);
   }
  }
  const elapsed=(Date.now()-ready)/1000;
  if(elapsed>scene.seconds+.5)throw Error(`${scene.id} exceeded its editorial duration (${elapsed}s). The real result will not be substituted.`);
  await delay(Math.max(0,scene.seconds*1000-(Date.now()-ready))+650);
  await page.screenshot({path:join(dir,'last-frame.png')});
  const evidence=await page.evaluate(()=>window.film?film.state.evidence:[]);
  const finalURL=page.url();const video=page.video();await context.close();const raw=await video.path();
  frames.push({...scene,raw,trimStart,evidence,errors,url:finalURL.replace(demo.url,'http://127.0.0.1:[demo-port]')});
  writeFileSync(join(out,'recording.json'),JSON.stringify({width:1920,height:1080,scenes:frames,artifact:{sha256:sha(resolve('dist/mirabase-windows-amd64.exe')),nativeSHA256:sha(resolve('internal/mbengine/platform/payload/windows_amd64/mirabase_engine.dll'))},qualification:'Recording evidence only; release qualification is separate.'},null,2));
 }
}finally{if(browser)await browser.close();if(demo)await demo.stop();}
