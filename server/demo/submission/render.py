"""Edit existing evidence footage. Starts no server and creates no databases."""
import json, subprocess, hashlib, textwrap
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[3]
SERVER = ROOT / 'server'
WORK = ROOT / '_build/submission-edit'
DEST = SERVER / 'dist/submission'
DEST.mkdir(parents=True, exist_ok=True)
STORY = json.loads((Path(__file__).parent / 'story.json').read_text(encoding='utf-8'))
OLD = json.loads((ROOT / '_build/hackathon-film/recording.json').read_text(encoding='utf-8'))
SOURCES = {s['id']: s for s in OLD['scenes']}
NAVY, PANEL, MINT, BLUE, WHITE, MUTED = '#091727', '#10263B', '#71F3C1', '#8DBAFF', '#F4F8FF', '#A4B6C9'

def font(n, bold=False):
    return ImageFont.truetype('C:/Windows/Fonts/' + ('segoeuib.ttf' if bold else 'segoeui.ttf'), n)

def text(d, xy, t, n=30, fill=WHITE, bold=False):
    d.text(xy, t, font=font(n, bold), fill=fill)

def lines(d, xy, t, n=30, width=80, fill=MUTED, bold=False):
    for k, line in enumerate(textwrap.wrap(t, width)):
        text(d, (xy[0], xy[1] + k * n * 1.35), line, n, fill, bold)

def box(d, rect):
    d.rounded_rectangle(rect, 20, fill=PANEL, outline='#264059', width=2)

def draw_card(d, s):
    sid = s['id']
    if sid == 'hook':
        text(d, (88, 223), 'A private database for every', 67, WHITE, True)
        text(d, (88, 310), 'user, tenant, and agent.', 67, MINT, True)
        names = [('ALICE', 'Personal workspace'), ('BOB', 'Personal workspace'), ('NORTHSTAR', 'Shared tenant workspace'), ('RESEARCH AGENT', 'Agent-owned data')]
        for i, (name, detail) in enumerate(names):
            x = 88 + 440 * i
            box(d, (x, 476, x + 416, 682))
            text(d, (x + 28, 509), name, 27, BLUE, True)
            d.rectangle((x + 29, 578, x + 89, 619), fill='#174E48', outline=MINT, width=2)
            d.ellipse((x + 29, 567, x + 89, 589), fill='#174E48', outline=MINT, width=2)
            d.ellipse((x + 29, 607, x + 89, 629), fill='#174E48', outline=MINT, width=2)
            text(d, (x + 112, 575), 'Private DB', 27, WHITE, True)
            text(d, (x + 28, 643), detail, 22, MUTED)
            d.line((x + 208, 683, x + 208, 739), fill='#4A786E', width=3)
        box(d, (88, 740, 1824, 864))
        text(d, (130, 772), 'ONE MIRABASE PROCESS', 31, MINT, True)
        text(d, (740, 776), 'Shared backend services. In-process Turso.', 30)
        text(d, (90, 911), 'Self-hosted  /  Go  /  PocketBase foundation + Turso engine', 27, MUTED)
    elif sid == 'engineering':
        box(d, (72, 213, 920, 900))
        box(d, (960, 213, 1848, 900))
        text(d, (112, 248), 'THE BACKEND', 25, MINT, True)
        text(d, (112, 300), 'Go + in-process Turso', 43, WHITE, True)
        for y, a, b in [(400, 'One process', 'Shared services, many database files'), (515, 'Native payload integrity', 'Compiled-in SHA-256 trust pins'), (630, 'Real acceptance evidence', 'Requests, failures, artifacts and tests')]:
            text(d, (112, y), a, 31, WHITE, True)
            lines(d, (112, y + 48), b, 27, 45)
        text(d, (112, 827), 'Release qualification remains open.', 25, BLUE)
        text(d, (1002, 248), 'AI BUILD TOOLING', 25, MINT, True)
        text(d, (1002, 300), 'Muse Spark 1.3', 43, WHITE, True)
        lines(d, (1002, 375), 'Used to build MiraBase and the separate formal-verification companion.', 29, 47)
        entries = [('CLAIM → MODEL', 'Executable Quint specifications'), ('CHECK → CHALLENGE', 'Simulation, bounded checks, adversarial probes'), ('EVIDENCE → LIMITS', 'Provenance, counterexamples and explicit bounds')]
        for i, (a, b) in enumerate(entries):
            y = 500 + i * 105
            text(d, (1002, y), a, 26, BLUE, True)
            text(d, (1002, y + 42), b, 25)
        text(d, (1002, 841), 'github.com/lordolami/formal-verification', 25, MUTED)
    elif sid == 'roadmap':
        labels = [('01', 'QUALIFY THE STANDALONE', ['Collection-policy parity', 'Platform, SDK and recovery acceptance', 'Signing and reproducible delivery']), ('02', 'COMPLETE THE LOCAL EXPERIENCE', ['Offline clients + native sync', 'Full AgentFS integration', 'Investigate Wasm + wazero packaging']), ('03', 'DISTRIBUTION UNDERNEATH', ['Managed placement + replication', 'Routing, balancing + fenced failover', 'Console, metering + recovery'])]
        for i, (num, title, items) in enumerate(labels):
            x = 72 + i * 600
            box(d, (x, 225, x + 576, 858))
            text(d, (x + 30, 255), num, 67, MINT, True)
            lines(d, (x + 30, 366), title, 32, 26, WHITE, True)
            for j, line in enumerate(items):
                text(d, (x + 30, 493 + j * 99), '•', 27, MINT)
                lines(d, (x + 58, 493 + j * 99), line, 27, 29)
            text(d, (x + 30, 803), 'PLANNED / NOT CLAIMED SHIPPED', 20, BLUE, True)
        text(d, (78, 910), 'Keep the application and private-database model as the platform grows.', 31)
    elif sid == 'outro':
        text(d, (87, 236), 'Build your frontend.', 94, WHITE, True)
        text(d, (87, 350), 'Own your backend.', 94, MINT, True)
        lines(d, (92, 523), 'One backend. A private database for every user, tenant, and agent.', 44, 70, WHITE)
        text(d, (94, 714), 'MiraBase', 52, WHITE, True)
        text(d, (94, 791), 'Built with Muse Spark 1.3', 30, BLUE)
        text(d, (94, 854), 'Extending PocketBase with Turso · Independent project', 25, MUTED)
        text(d, (94, 910), 'Working prototype · qualification and planned features documented', 25, MUTED)

def base(s, idx):
    im = Image.new('RGB', (1920, 1080), NAVY)
    d = ImageDraw.Draw(im)
    for y in range(1080):
        d.line((0, y, 1920, y), fill=(9 + int(4*y/1080), 23 + int(8*y/1080), 39 + int(12*y/1080)))
    text(d, (72, 28), 'm  MiraBase', 32, WHITE, True)
    text(d, (1530, 34), f'{idx+1:02d} / 11     DEMO', 21, MUTED)
    text(d, (72, 91), s['title'], 48, WHITE, True)
    d.line((72, 165, 1848, 165), fill='#294158', width=2)
    if s['kind'] != 'card':
        label = 'ACTUAL DASHBOARD · DESKTOP RECORDING' if s['kind'] == 'desktop' else 'CONNECTED EXAMPLE FRONTEND · ACTUAL SERVER RESPONSES'
        if s['id'] == 'operations':
            label = 'CONNECTED EXAMPLE → ACTUAL POCKETBASE IMPORT DASHBOARD'
        text(d, (74, 935), label, 21, MINT, True)
        if s['id'] == 'workspace':
            text(d, (1175, 935), 'OPERATOR CREDENTIALS · SELECTED WORKSPACE FEATURES', 19, MUTED)
    else:
        draw_card(d, s)
    for i in range(11):
        d.rounded_rectangle((72+i*162, 1060, 218+i*162, 1064), 2, fill=MINT if i <= idx else '#243C50')
    im.save(WORK / (s['id'] + '-base.png'))

def run(args, cwd=WORK):
    p = subprocess.run(args, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, encoding='utf-8', errors='replace')
    if p.returncode:
        raise RuntimeError(p.stderr[-5000:])
    return p.stdout

def ff(args):
    return run(['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y'] + args)

def duration(p):
    return float(run(['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', str(p)]).strip())

def stamp(t, ass=False):
    v = round(t * (100 if ass else 1000))
    u = 100 if ass else 1000
    h, v = divmod(v, 3600*u)
    m, v = divmod(v, 60*u)
    sec, f = divmod(v, u)
    return f'{h}:{m:02}:{sec:02}.{f:02}' if ass else f'{h:02}:{m:02}:{sec:02},{f:03}'

HEADER = '''[Script Info]
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080
ScaledBorderAndShadow: yes
[V4+ Styles]
Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding
Style: Default,Segoe UI,32,&H00FFFFFF,&H000000FF,&H00271709,&H00271709,0,0,0,0,100,100,0,0,3,7,0,2,100,100,48,1
Style: Callout,Segoe UI,27,&H00C1F371,&H000000FF,&H00271709,&H00271709,-1,0,0,0,100,100,0,0,3,9,0,2,60,60,155,1
[Events]
Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text
'''
CALLS = {
    'databases': [(0,4,'Registry: four demo databases'),(4,8,'Alice → user database'),(8,12,'Bob → different user database'),(12,16,'Northstar → shared tenant database'),(16,20,'Research agent → agent database')],
    'access': [(0,1.6,'Attempt a cross-database read'),(1.6,6.5,'Bob → Alice: 403 DENIED'),(6.5,11.5,'Operator adds an explicit read grant'),(11.5,16.5,'Bob → Alice: 200 OK'),(16.5,22,'Read grant revoked → Bob is denied again')],
    'workspace': [(0,4.8,'Create an operator-managed workspace'),(4.8,8.8,'Write the original note'),(8.8,12.9,'Save a snapshot'),(12.9,17.1,'Replace the current contents'),(17.1,22,'Restore → original file bytes return')],
}
CAPTIONS, META = [], []
elapsed, capnum = 0, 1
for idx, s in enumerate(STORY):
    sid, seconds = s['id'], s['seconds']
    base(s, idx)
    visual = WORK / (sid + ('-visual-v2.mp4' if sid in ['foundation', 'operations'] else '-visual.mp4'))
    common = ['-an','-c:v','libx264','-preset','veryfast','-crf','19','-pix_fmt','yuv420p','-threads','2']
    if not visual.exists():
        if s['kind'] == 'card':
            ff(['-loop','1','-framerate','25','-i',str(WORK/(sid+'-base.png')),'-t',str(seconds)] + common + [str(visual)])
        else:
            if sid == 'foundation':
                desktop = SERVER / 'dist/screen-recording/MiraBase-Real-Screen-Session.mkv'
                parts = []
                for k, (start, length) in enumerate([(168,4),(254,5),(260,4),(296,4)]):
                    part = WORK / f'desktop-part-{k}.mp4'
                    ff(['-ss',str(start),'-i',str(desktop),'-t',str(length),'-vf','crop=1366:488:0:144,scale=1776:-2,pad=1776:734:0:(oh-ih)/2:color=0x10263B,fps=25'] + common + [str(part)])
                    parts.append(part)
                (WORK/'desktop-concat.txt').write_text('\n'.join("file '"+str(p).replace('\\','/')+"'" for p in parts),encoding='utf-8')
                cropped = WORK / 'desktop-edited.mp4'
                ff(['-f','concat','-safe','0','-i',str(WORK/'desktop-concat.txt'),'-c','copy',str(cropped)])
                source, start, length, crop = cropped, 0, seconds, 'null'
            elif sid == 'operations':
                parts = []
                for k, (source_id, length) in enumerate([('operations', 7), ('import', 5)]):
                    old = SOURCES[source_id]
                    part = WORK / f'operator-part-{k}.mp4'
                    video_filter = 'crop=1600:660:280:275,scale=1776:734' if k == 0 else 'crop=1920:794:0:180,scale=1776:734'
                    source_start = old['trimStart'] + (2 if k == 1 else 0)
                    ff(['-ss',str(source_start),'-i',old['raw'],'-t',str(length),'-vf',video_filter+',fps=25'] + common + [str(part)])
                    parts.append(part)
                (WORK/'operator-concat.txt').write_text('\n'.join("file '"+str(p).replace('\\','/')+"'" for p in parts),encoding='utf-8')
                cropped = WORK / 'operator-edited.mp4'
                ff(['-f','concat','-safe','0','-i',str(WORK/'operator-concat.txt'),'-c','copy',str(cropped)])
                source, start, length, crop = cropped, 0, seconds, 'null'
            else:
                old = SOURCES[s['source']]
                source, start, length = Path(old['raw']), old['trimStart'], old['seconds']
                crop = 'crop=1600:660:280:275,scale=1776:734'
            ff(['-loop','1','-framerate','25','-i',str(WORK/(sid+'-base.png')),'-ss',str(start),'-t',str(length),'-i',str(source),'-filter_complex',f'[1:v]{crop},setpts={(seconds/length):.8f}*(PTS-STARTPTS),fps=25,tpad=stop_mode=clone:stop_duration=2[screen];[0:v][screen]overlay=72:182:shortest=1[v]','-map','[v]','-t',str(seconds)] + common + [str(visual)])
        print('Visual rendered:', sid, flush=True)
    wave = WORK / 'audio' / (sid + '.wav')
    timing = []
    if s.get('segments'):
        pieces = []
        for k, segment in enumerate(s['segments']):
            source_wave = WORK / 'audio' / f'{sid}-{k}.wav'
            length = segment['end'] - segment['start']
            natural = duration(source_wave)
            pace = max(.9, natural/(length-.35))
            if pace > 1.22:
                raise RuntimeError(f'Rewrite overlong narration: {sid}-{k}')
            piece = WORK / 'audio' / f'{sid}-{k}-timed.wav'
            ff(['-i',str(source_wave),'-af',f'atempo={pace:.8f},adelay=150,apad,atrim=duration={length}','-ar','48000',str(piece)])
            pieces.append(piece)
            timing.append((segment['start']+.15,natural/pace,segment['text']))
        listing = WORK / f'{sid}-audio.txt'
        listing.write_text('\n'.join("file '"+str(p).replace('\\','/')+"'" for p in pieces),encoding='utf-8')
        ff(['-f','concat','-safe','0','-i',str(listing),'-c:a','pcm_s16le',str(wave)])
        audio_length, tempo = duration(wave), 1
        audio_delay = 0
    else:
        audio_length = duration(wave)
        tempo = max(.88, min(1.18, audio_length / (seconds-1.05)))
        spoken = audio_length / tempo
        if spoken > seconds-.6:
            raise RuntimeError('Narration too long: ' + sid)
        timing.append((.4,spoken,s['text']))
        audio_delay = 400
    events = []
    for speech_start, speech_length, speech_text in timing:
        words, i = speech_text.split(), 0
        while i < len(words):
            count = min(12, len(words)-i)
            for j in range(5, count+1):
                if words[i+j-1].endswith(('.', '?', '!')):
                    count = j
                    break
            a = speech_start + speech_length*i/len(words)
            b = min(seconds-.15, speech_start+speech_length*(i+count)/len(words))
            t = ' '.join(words[i:i+count])
            events.append(f'Dialogue: 0,{stamp(a,True)},{stamp(b,True)},Default,,0,0,0,,{t}')
            CAPTIONS.append(f'{capnum}\n{stamp(elapsed+a)} --> {stamp(elapsed+b)}\n{t}\n')
            capnum += 1
            i += count
    for a,b,t in CALLS.get(sid,[]):
        events.append(f'Dialogue: 1,{stamp(a,True)},{stamp(b,True)},Callout,,0,0,0,,{t}')
    (WORK/(sid+'.ass')).write_text(HEADER+'\n'.join(events)+'\n',encoding='utf-8')
    clip = WORK / (sid + '.mp4')
    # Audio is retained explicitly; the visual helper's -an option is omitted here.
    ff(['-i',str(visual),'-i',str(wave),'-vf',f'ass={sid}.ass,fade=t=in:st=0:d=0.15,fade=t=out:st={seconds-.15}:d=0.15','-af',f'atempo={tempo:.8f},adelay={audio_delay},apad,atrim=duration={seconds},loudnorm=I=-16:TP=-1.5:LRA=7','-t',str(seconds)] + common[1:] + ['-c:a','aac','-b:a','128k','-ar','48000','-movflags','+faststart',str(clip)])
    META.append({'id':sid,'title':s['title'],'start':elapsed,'end':elapsed+seconds,'kind':s['kind'],'sourceScene':s.get('source'),'narrationSeconds':audio_length,'tempo':tempo})
    elapsed += seconds
    print('Chapter complete:',sid,flush=True)

(DEST/'MiraBase-Submission-3min.srt').write_text('\n'.join(CAPTIONS),encoding='utf-8')
(WORK/'final-concat.txt').write_text('\n'.join("file '"+str(WORK/(s['id']+'.mp4')).replace('\\','/')+"'" for s in STORY),encoding='utf-8')
chapters = '\n'.join(f"[CHAPTER]\nTIMEBASE=1/1000\nSTART={int(m['start']*1000)}\nEND={int(m['end']*1000)}\ntitle={m['title']}" for m in META)
(WORK/'metadata.txt').write_text(';FFMETADATA1\ntitle=MiraBase — One backend. Private databases.\ncomment=Edited actual footage. Connected example client labelled. Roadmap planned.\n'+chapters,encoding='utf-8')
(DEST/'SUBMISSION.md').write_text((Path(__file__).parent/'SUBMISSION.md').read_text(encoding='utf-8'),encoding='utf-8')
video = DEST / 'MiraBase-Submission-3min.mp4'
ff(['-f','concat','-safe','0','-i',str(WORK/'final-concat.txt'),'-i',str(WORK/'metadata.txt'),'-map_metadata','1','-map_chapters','1','-vf','fps=25:start_time=0','-af','aresample=async=1:first_pts=0,atrim=duration=180','-t','180','-c:v','libx264','-preset','veryfast','-crf','19','-threads','2','-pix_fmt','yuv420p','-c:a','aac','-b:a','128k','-ar','48000','-movflags','+faststart',str(video)])
sha = lambda p: hashlib.sha256(Path(p).read_bytes()).hexdigest()
probe = json.loads(run(['ffprobe','-v','error','-show_format','-show_streams','-show_chapters','-of','json',str(video)]))
assert abs(float(probe['format']['duration'])-180) < .05
assert probe['streams'][0]['width'] == 1920 and probe['streams'][0]['height'] == 1080
evidence = {'videoSHA256':sha(video),'originalVideoSHA256':sha(SERVER/'dist/demo/MiraBase-Hackathon-Demo-3min.mp4'),'desktopSourceSHA256':sha(SERVER/'dist/screen-recording/MiraBase-Real-Screen-Session.mkv'),'recordingManifestSHA256':sha(ROOT/'_build/hackathon-film/recording.json'),'artifact':OLD['artifact'],'chapters':META,'probe':probe,'limits':['Edited recorded demonstration, not a new live run.','No bulk job or server was started for this edit.','Connected example frontend is not the bundled dashboard.','HTTP grant evidence demonstrates database ownership/grants, not complete collection-policy parity.','Workspace demonstration uses operator credentials; not full AgentFS.','Cloud, full offline sync, Wasm packaging and full AgentFS are planned.','Muse Spark credit is author-supplied build provenance, not independently measured model contribution.','Separate Quint harness capabilities do not prove this entire backend.','Release remains unqualified; consult docs/launch/status.md.'],'evidenceBySourceScene':{sid:SOURCES[sid]['evidence'] for sid in ['databases','access','search','live','workspace','operations','import']}}
(DEST/'evidence.json').write_text(json.dumps(evidence,indent=2),encoding='utf-8')
(DEST/'transcript.md').write_text('# MiraBase — three-minute submission film\n\n'+'\n\n'.join(f"## {int(m['start']//60):02}:{int(m['start']%60):02} — {s['title']}\n\n{s['text']}" for s,m in zip(STORY,META)),encoding='utf-8')
ff(['-ss','32','-i',str(video),'-frames:v','1',str(DEST/'preview.png')])
print(json.dumps({'video':str(video),'duration':probe['format']['duration'],'sha256':sha(video)},indent=2),flush=True)
