import {startDemo,delay} from './fixture.mjs';
import {runPostgres} from './external.mjs';
import {mkdirSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {pathToFileURL} from 'node:url';
const root=resolve(process.env.MIRABASE_WORKSPACE||'..');
let playwright;try{playwright=await import('playwright-core');}catch{playwright=await import(pathToFileURL(join(root,'_build/pwtools/node_modules/playwright-core/index.mjs')));}
const {chromium}=playwright;
const out=join(root,'_build/film-probe');mkdirSync(out,{recursive:true});
let demo,browser;const results=[];
try{
 demo=await startDemo();
 browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true,args:['--disable-background-networking']});
 const page=await browser.newPage({viewport:{width:1920,height:1080},deviceScaleFactor:1});
 await page.exposeFunction('runExternalPG',()=>runPostgres(demo));
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(demo.url);await page.waitForFunction(()=>!!window.film);
 await page.evaluate(credentials=>film.authenticate(credentials),{email:demo.email,password:demo.password,memberPassword:demo.memberPassword});
 const sections=await page.evaluate(()=>film.sections.map(s=>s[0]));
 for(const section of sections){
  await page.locator(`[data-section="${section}"]`).click();
  for(const id of await page.locator('[data-action]').evaluateAll(nodes=>nodes.map(n=>n.dataset.action))){
   const b=page.locator(`[data-action="${id}"]`);await b.click();await page.waitForFunction(id=>{const b=document.querySelector(`[data-action="${id}"]`);return b.dataset.result},id,{timeout:60000});
   const result=await b.getAttribute('data-result');results.push({section,action:id,result});console.log(section,id,result);
   await delay(['realtime','realtime-change','live-stream','live-mutate','refresh-reference'].includes(id)?2500:120);
  }
  await page.screenshot({path:join(out,section+'.png')});
 }
 writeFileSync(join(out,'evidence.json'),JSON.stringify({results,errors,evidence:await page.evaluate(()=>film.state.evidence),artifact:demo.root},null,2));
 if(results.some(r=>r.result!=='passed')||errors.length)process.exitCode=1;
}finally{if(browser)await browser.close();if(demo)await demo.stop();}
