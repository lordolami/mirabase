import {readFileSync,writeFileSync,mkdirSync,copyFileSync} from 'node:fs';
import {spawnSync} from 'node:child_process';
import {resolve,join} from 'node:path';
import {createHash} from 'node:crypto';
const root=resolve('..'),out=join(root,'_build/hackathon-film');
const recording=JSON.parse(readFileSync(join(out,'recording.json'),'utf8'));
if(recording.scenes.reduce((n,s)=>n+s.seconds,0)!==180||recording.scenes.length!==20)throw Error('Recording incomplete.');
const destination=resolve('dist/demo');mkdirSync(destination,{recursive:true});
function run(cmd,args,cwd=out){const p=spawnSync(cmd,args,{cwd,encoding:'utf8',windowsHide:true,maxBuffer:8*1024*1024});if(p.status!==0)throw Error(`${cmd} failed: ${p.stderr.slice(-5000)}`);return p.stdout;}
function stamp(seconds,ass=false){let n=Math.round(seconds*(ass?100:1000));const unit=ass?100:1000;const h=Math.floor(n/(3600*unit));n%=3600*unit;const m=Math.floor(n/(60*unit));n%=60*unit;const s=Math.floor(n/unit),fraction=n%unit;return `${ass?h:String(h).padStart(2,'0')}:${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}${ass?'.':','}${String(fraction).padStart(ass?2:3,'0')}`;}
const assHeader=`[Script Info]\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\nScaledBorderAndShadow: yes\n[V4+ Styles]\nFormat: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding\nStyle: Default,Segoe UI,32,&H00FFFFFF,&H000000FF,&H00111927,&H90111927,0,0,0,0,100,100,0,0,3,12,0,2,130,130,33,1\n[Events]\nFormat: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n`;
const captions=[],chapters=[];let offset=0,index=1;const files=[];
for(const scene of recording.scenes){
 const dir=join(out,scene.id),wave=join(root,'_build/film-audio',scene.id+'.wav');
 const audioSeconds=Number(run('ffprobe',['-v','error','-show_entries','format=duration','-of','default=nw=1:nk=1',wave]).trim());
 const tempo=Math.max(1,audioSeconds/(scene.seconds-.65));if(tempo>1.2)throw Error('Narration needs rewriting: '+scene.id);
 const spoken=audioSeconds/tempo,words=scene.text.split(/\s+/),blocks=[];
 for(let i=0;i<words.length;){let count=Math.min(10,words.length-i);if(words.length-i-count<3)count=words.length-i;const line=words.slice(i,i+count).join(' '),start=.25+spoken*i/words.length,end=Math.min(scene.seconds-.15,.25+spoken*(i+count)/words.length);blocks.push(`Dialogue: 0,${stamp(start,true)},${stamp(end,true)},Default,,0,0,0,,${line.replaceAll('{','\\{')}`);captions.push(`${index++}\n${stamp(offset+start)} --> ${stamp(offset+end)}\n${line}\n`);i+=count;}
 writeFileSync(join(dir,'captions.ass'),assHeader+blocks.join('\n')+'\n','utf8');
 const clip=join(dir,'clip.mp4');
 run('ffmpeg',['-hide_banner','-loglevel','error','-y','-ss',scene.trimStart.toFixed(3),'-i',scene.raw,'-i',wave,'-vf',`fps=25,ass=captions.ass,fade=t=in:st=0:d=0.12,fade=t=out:st=${(scene.seconds-.12).toFixed(2)}:d=0.12`,'-af',`atempo=${tempo.toFixed(6)},adelay=250,apad,atrim=duration=${scene.seconds},loudnorm=I=-16:TP=-1.5:LRA=7`,'-t',String(scene.seconds),'-c:v','libx264','-preset','veryfast','-crf','20','-pix_fmt','yuv420p','-threads','2','-c:a','aac','-b:a','128k','-ar','48000','-movflags','+faststart',clip],dir);
 files.push(`file '${clip.replaceAll('\\','/')}'`);chapters.push(`[CHAPTER]\nTIMEBASE=1/1000\nSTART=${offset*1000}\nEND=${(offset+scene.seconds)*1000}\ntitle=${scene.title}`);offset+=scene.seconds;
 console.log('Encoded '+scene.id);
}
writeFileSync(join(out,'concat.txt'),files.join('\n')+'\n');
writeFileSync(join(out,'chapters.txt'),`;FFMETADATA1\ntitle=MiraBase — One backend. Private databases.\ncomment=Actual application footage with narrated demonstrations and explicitly labelled roadmap.\n${chapters.join('\n')}\n`);
const video=join(destination,'MiraBase-Hackathon-Demo-3min.mp4');
run('ffmpeg',['-hide_banner','-loglevel','error','-y','-f','concat','-safe','0','-i',join(out,'concat.txt'),'-i',join(out,'chapters.txt'),'-map_metadata','1','-map_chapters','1','-vf','fps=25:start_time=0','-af','aresample=async=1:first_pts=0,atrim=duration=180','-c:v','libx264','-preset','veryfast','-crf','19','-pix_fmt','yuv420p','-threads','2','-c:a','aac','-b:a','128k','-ar','48000','-t','180','-movflags','+faststart',video]);
const srt=join(destination,'MiraBase-Hackathon-Demo-3min.srt');writeFileSync(srt,captions.join('\n'));
writeFileSync(join(destination,'transcript.md'),'# MiraBase — three-minute hackathon demo\n\n'+recording.scenes.map(s=>`## ${s.title}\n\n${s.text}\n`).join('\n'));
const sha=path=>createHash('sha256').update(readFileSync(path)).digest('hex');
const probe=JSON.parse(run('ffprobe',['-v','error','-show_streams','-show_format','-show_chapters','-of','json',video]));
const coverage=recording.scenes.map(s=>({id:s.id,title:s.title,seconds:s.seconds,kind:['roadmap'].includes(s.id)?'planned roadmap graphic':['outro'].includes(s.id)?'credits':['import','dashboard'].includes(s.id)?'actual admin dashboard':'actual connected example client',actions:s.actions||[],evidenceCount:s.evidence.length,pageErrors:s.errors}));
writeFileSync(join(destination,'evidence.json'),JSON.stringify({durationSeconds:180,videoSHA256:sha(video),subtitleSHA256:sha(srt),artifact:recording.artifact,coverage,probe,limits:'A three-minute feature overview, not an exhaustive acceptance demonstration. Full release qualification remains separate. Roadmap is planned, not deployed. Spark credit reflects the project author’s stated development tooling.'},null,2));
copyFileSync(join(out,'databases','last-frame.png'),join(destination,'preview.png'));
console.log(JSON.stringify({video,duration:probe.format.duration,width:probe.streams[0].width,height:probe.streams[0].height,sha256:sha(video)}));
