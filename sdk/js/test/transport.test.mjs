import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {once} from 'node:events';
import {forApplication} from '../dist/index.js';

async function withServer(run) {
 const server=createServer((req,res)=>{res.setHeader('content-type','application/json');res.end(JSON.stringify(req.headers));});
 server.listen(0,'127.0.0.1'); await once(server,'listening');
 try { await run(`http://127.0.0.1:${server.address().port}`); }
 finally { server.closeAllConnections(); await new Promise(resolve=>server.close(resolve)); }
}
test('immutable database selector overrides caller Headers without losing custom headers',()=>withServer(async url=>{
 const view=forApplication(url,()=> 'test-token').withContext({databaseId:'allowed'});
 const out=await view.request('/echo',{headers:new Headers({'X-MiraBase-Database':'wrong','X-Custom':'yes'})});
 assert.equal(out['x-mirabase-database'],'allowed');assert.equal(out['x-custom'],'yes');assert.equal(out.authorization,'test-token');
}));
test('tuple headers and JSON requests are normalized',()=>withServer(async url=>{
 const out=await forApplication(url).request('/echo',{method:'POST',headers:[['X-Custom','tuple']],body:JSON.stringify({value:1})});
 assert.equal(out['x-custom'],'tuple');assert.equal(out['content-type'],'application/json');
}));
test('FormData retains the generated multipart boundary',()=>withServer(async url=>{
 const body=new FormData();body.set('name','value');
 const out=await forApplication(url).request('/echo',{method:'POST',body});
 assert.match(out['content-type'],/^multipart\/form-data; boundary=/);
}));
test('request paths cannot redirect credentials outside the application',()=>withServer(async url=>{
 const api=forApplication(url+'/a/allowed',()=> 'test-token');
 await assert.rejects(api.request('@example.invalid/x'),/server-relative/);
 await assert.rejects(api.request('//example.invalid/x'),/server-relative/);
 await assert.rejects(api.request('/../../api/health'),/escaped/);
}));
