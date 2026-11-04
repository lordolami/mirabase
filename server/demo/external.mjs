import {createRequire} from 'node:module';
import {resolve} from 'node:path';

// This is a real external client, not a substituted HTTP response.
export async function runPostgres(demo){
 let require=createRequire(new URL('./package.json',import.meta.url));
 try{require.resolve('pg');}catch{require=createRequire(new URL('../../_build/pgcheck/package.json',import.meta.url));}
 const {Client}=require('pg');
 const d=demo.config.databases.alice,id=d.databaseId||d.id;
 const credential=await demo.call(`/api/mirabase/v1/databases/${id}/credentials`,'POST',{purpose:'pg',scopes:['sql.read'],expiresAt:new Date(Date.now()+3600000).toISOString()});
 const client=new Client({host:'127.0.0.1',port:demo.pgPort,user:credential.id,password:credential.secret,database:id,ssl:{ca:demo.pgCA,rejectUnauthorized:true},connectionTimeoutMillis:10000});
 try{
  await client.connect();
  const sql='SELECT title, body FROM docs ORDER BY title LIMIT 3';
  const simple=await client.query(sql);
  const prepared=await client.query({name:'demo_title',text:'SELECT title FROM docs WHERE title = $1',values:['hello MiraBase']});
  return {client:'node-postgres '+require('pg/package.json').version,connection:{tlsAuthorized:client.connection.stream.authorized,protocol:client.connection.stream.getProtocol(),database:id,scopes:['sql.read']},sql,rows:simple.rows,prepared:{sql:'SELECT title FROM docs WHERE title = $1',parameters:['hello MiraBase'],rows:prepared.rows}};
 }finally{await client.end();await demo.call(`/api/mirabase/v1/credentials/${credential.id}/revoke`,'POST');}
}

if(process.argv.includes('--probe')){
 const {startDemo}=await import('./fixture.mjs');let demo;
 try{demo=await startDemo();console.log(JSON.stringify(await runPostgres(demo),null,2));}
 finally{if(demo)await demo.stop();}
}
