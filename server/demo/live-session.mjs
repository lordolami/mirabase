import {startDemo,delay} from './fixture.mjs';
import {writeFileSync,mkdirSync,existsSync} from 'node:fs';
import {resolve,join} from 'node:path';
const output=resolve(process.argv[2]||'../_build/screen-session');mkdirSync(output,{recursive:true});
const demo=await startDemo();
writeFileSync(join(output,'private-session.json'),JSON.stringify({url:demo.url,root:demo.root,pid:demo.proc.pid,email:demo.email,password:demo.password,memberPassword:demo.memberPassword,adminToken:demo.adminToken,config:demo.config,pgPort:demo.pgPort,pgCA:demo.pgCA}),{mode:0o600});
console.log(JSON.stringify({ready:true,url:demo.url,pid:demo.proc.pid,privateSession:join(output,'private-session.json')}));
try{while(demo.proc.exitCode===null&&!existsSync(join(output,'STOP'))){await delay(1000);}}
finally{await demo.stop();}
