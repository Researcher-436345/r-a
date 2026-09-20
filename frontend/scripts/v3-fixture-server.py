"""Local, in-memory API fixtures for comparing v3 screens without a backend.
Run: python3 scripts/v3-fixture-server.py
Then: VITE_API_URL=http://127.0.0.1:8089 npm run dev -- --host 127.0.0.1 --port 5174
Sign in with any test email and an 8+ character test password. No real credentials.
All writes are in memory and disappear when this process exits.
"""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse, parse_qs
from datetime import datetime, timezone, timedelta
import json, re, uuid
ROOT = Path(__file__).resolve().parents[1]
source = (ROOT/'examples/v3/v3/Home.dc.html').read_text()
raw = re.findall(r"\{ id: '(.*?)', title: '(.*?)', date: '(.*?)', rel: '(.*?)', authors: '(.*?)', cat: '(.*?)', year: '(.*?)', cites: '(.*?)', abstract: '(.*?)' \}", source)
now = datetime.now(timezone.utc)
feed = [dict(arxiv_id=a, title=b, authors=e.split(', '), category=f, published_at=f'2026-09-0{8-i}T00:00:00Z', citation_count=int(h or 0), abstract=j, abs_url='https://arxiv.org/abs/'+a, pdf_url='', popularity_score=0) for i,(a,b,c,d,e,f,g,h,j) in enumerate(raw)]
papers = [dict(id='paper-'+str(i+1),title=p['title'],abstract=p['abstract'],authors=[dict(id=str(j),name=n) for j,n in enumerate(p['authors'])], arxiv_id=p['arxiv_id'],doi=None,year=2026,venue=None,created_at=now.isoformat(),latest_version=dict(id='v1',source='arxiv',status='ready',pdf_key='fixture.pdf',size_bytes=1000,error_message=None)) for i,p in enumerate(feed)]
folders=[dict(id='folder-'+str(i),name=n,parent_id='folder-3' if i==4 else None,system_key=k,article_count=c,created_at=now.isoformat()) for i,(n,k,c) in enumerate([('Хочу прочитать','want_to_read',12),('Читаю','reading',3),('Другое','other',7),('RL / Flow policies',None,5),('Test-time compute',None,2),('Агенты и инструменты',None,4)])]
# Use each library row verbatim, including the failed-PDF state.
from html import unescape
library_source = (ROOT/'examples/v3/v3/Library.dc.html').read_text()
library_papers=[]
for i,article in enumerate(re.findall(r'<article\b.*?</article>',library_source,re.S)):
 def value(pattern):
  match=re.search(pattern,article,re.S)
  return unescape(re.sub(r'<[^>]+>','',match[1]).strip()) if match else ''
 p=dict(papers[0]);p['id']='paper-'+str(i+1)
 p['title']=value(r'<a [^>]+>(.*?)</a>')
 p['abstract']=value(r'<p style="display:-webkit-box[^>]+>(.*?)</p>')
 p['authors']=[dict(id=str(j),name=n) for j,n in enumerate(value(r'<div style="margin:7px 0 14px[^>]+>(.*?)</div>').split(', '))]
 p['arxiv_id']=value(r'arXiv:([^<]+)') or None;p['doi']=value(r'DOI:([^<]+)') or None
 p['latest_version']=dict(papers[0]['latest_version'])
 if 'источник вернул 403' in article:p['latest_version'].update(status='failed',error_message='Не удалось получить PDF: источник вернул 403.')
 library_papers.append(p)
papers=library_papers
items=[dict(id='item-'+str(i),paper=p,status='unread',favorite=False,folder_id='folder-0',added_at=now.isoformat()) for i,p in enumerate(papers)]
chats=[dict(id='chat-'+str(i),title=t,mode='web',created_at=(now-timedelta(days=0 if i<2 else 2)).isoformat(),updated_at=(now-timedelta(days=0 if i<2 else 2)).isoformat()) for i,t in enumerate(['Производительный веб-поиск','Свежие методы RLHF: обзор','Какие статьи стоит прочитать по агентам?','Sparse attention для длинного контекста','Flow matching vs diffusion в RL'])]
answer='''За последние полгода сдвиг очевиден: индустрия уходит от классического PPO к более простым и стабильным схемам. Три направления показывают устойчивый выигрыш.

### 1. Прямая оптимизация предпочтений

DPO и его вариации (IPO, KTO, SimPO) убирают reward-модель и RL-цикл целиком. [SimPO](https://arxiv.org/abs/2405.14734) добавляет нормировку по длине и даёт +4–6 п.п. на AlpacaEval 2 при том же бюджете.

### 2. GRPO и групповые оценки

Вместо value-сети — нормировка награды внутри группы сэмплов. [DeepSeek-R1](https://arxiv.org/abs/2501.12948) показал, что на задачах с проверяемым ответом этого достаточно для сильного reasoning.

### 3. Верифицируемые награды

RLVR заменяет обученную reward-модель детерминированной проверкой (тесты, символьная верификация). Это снимает reward hacking, но ограничено доменами с ясным критерием.'''
def message(role, content, chat='chat-0'):
 return dict(id=str(uuid.uuid4()),chat_id=chat,role=role,content=content,sources=[],created_at=now.isoformat())
messages={'chat-0':[message('user','Собери обзор по свежим методам RLHF за последние полгода — что реально работает лучше PPO?'),message('assistant',answer)]}
notes=[dict(id='note-'+str(i),paper_id='paper-1',page=1,rect=None,selected_text=q,note=n,color=c,created_at=now.isoformat(),updated_at=now.isoformat()) for i,(q,n,c) in enumerate([('…incorporating them into RL pipelines for policy improvement has proven more difficult.','Ключевая мотивация: flow/diffusion-политики плохо встраиваются в RL из-за нестабильности обучения актора.','#f2d35c'),('…using the value gradient to guide the reference policy to generate higher-value actions.','Проверить вывод оценки градиента критика в разделе 5 — кажется, тут вся новизна.','#7fcf9e')])]
class Handler(BaseHTTPRequestHandler):
 def log_message(self, *args): pass
 def send(self,data,status=200,mime='application/json'):
  body=json.dumps(data,ensure_ascii=False).encode() if mime=='application/json' else data
  self.send_response(status); self.send_header('Content-Type',mime); self.send_header('Access-Control-Allow-Origin','http://127.0.0.1:5174'); self.send_header('Access-Control-Allow-Credentials','true'); self.send_header('Access-Control-Allow-Headers','Content-Type, Authorization'); self.send_header('Access-Control-Allow-Methods','GET, POST, PATCH, DELETE, OPTIONS'); self.send_header('Content-Length',str(len(body))); self.end_headers(); self.wfile.write(body)
 def do_OPTIONS(self):self.send({})
 def do_GET(self):self.dispatch()
 def do_POST(self):self.dispatch()
 def do_PATCH(self):self.dispatch()
 def do_DELETE(self):self.dispatch()
 def dispatch(self):
  path=urlparse(self.path).path; query=parse_qs(urlparse(self.path).query)
  body={}
  if self.command in ['POST','PATCH']:
   data=self.rfile.read(int(self.headers.get('Content-Length',0)))
   if 'application/json' in self.headers.get('Content-Type',''):body=json.loads(data or '{}')
  if path=='/auth/refresh':return self.send(dict(detail='Not authenticated'),401)
  if path=='/auth/logout':return self.send({})
  if path.startswith('/auth/'):
   return self.send(dict(access_token='local-visual-fixture',token_type='bearer'))
  public = path=='/feed/trending' or path=='/papers/arxiv/open' or (self.command=='GET' and re.fullmatch(r'/papers/[^/]+(?:/pdf(?:-url)?)?',path)) or path.endswith('/translate')
  if not public and not self.headers.get('Authorization'):return self.send(dict(detail='Not authenticated'),401)
  if path=='/papers/arxiv/open':return self.send(next((p for p in papers if p['arxiv_id']==body.get('arxiv_id')),papers[0]))
  if path=='/feed/trending':return self.send(dict(items=feed,category='cs.AI',cached=True))
  if path=='/library/folders':
   if self.command=='POST':
    folder=dict(id=str(uuid.uuid4()),name=body['name'],parent_id=body.get('parent_id'),system_key=None,article_count=0,created_at=now.isoformat());folders.append(folder);return self.send(folder)
   return self.send(dict(items=folders))
  if path=='/library':return self.send(dict(items=items if query.get('folder_id',[''])[0]=='folder-0' else [],page=1,limit=100,total=12 if items else 0))
  if path.startswith('/library/'):
   item=next((v for v in items if v['paper']['id']==path.split('/')[-1]),items[0] if items else {})
   if self.command=='PATCH':item.update(body)
   if self.command=='DELETE' and item in items:items.remove(item)
   return self.send(item)
  if path=='/search/chats':return self.send(chats)
  if path.startswith('/search/chats/'):
   cid=path.split('/')[3]; chat=next((v for v in chats if v['id']==cid),None)
   if self.command=='POST':
    if not chat:
     chat=dict(id=cid,title=body['message'],mode=body.get('mode','web'),created_at=now.isoformat(),updated_at=now.isoformat());chats.insert(0,chat)
    result=message('assistant',answer,cid);messages.setdefault(cid,[]).extend([message('user',body['message'],cid),result]);data='event: delta\ndata: '+json.dumps(dict(content=answer))+'\n\nevent: done\ndata: '+json.dumps(result)+'\n\n';return self.send(data.encode(),mime='text/event-stream')
   if self.command=='DELETE':
    if chat:chats.remove(chat)
    return self.send({})
   if not chat:return self.send(dict(detail='Chat not found'),404)
   return self.send(dict(**chat,messages=messages.get(cid,[])))
  if path=='/assistant/models':return self.send(dict(default='fixture',items=[dict(id='fixture',label='Fixture')]))
  if path.endswith('/chat/context'):return self.send(dict(used_tokens=41200,limit_tokens=128000,percent=32,paper_tokens=40000,history_tokens=1200,has_full_paper=True,model='fixture'))
  if path.endswith('/summary'):
   if self.command=='GET':return self.send(dict(detail='Summary has not been generated yet'),404)
   content = '# Тестовый обзор статьи\n\n## TL;DR\nКритик направляет генерацию действий.\n\n## Problem\n- Обучение flow-политики нестабильно.\n\n## Method\n- Градиент критика улучшает действия.\n\n## Results\n- Метод проверен на задачах управления.\n\n## Takeaways\n- Направление по ценности помогает политике.\n\n## Limitations\n- Это демонстрационные данные.\n\n## Deep dive\nПодробный тестовый разбор [p.1].'
   summary=dict(paper_id=path.split('/')[2],lang=query.get('lang',['ru'])[0],model='fixture',content=content,status='ready',error_message=None,updated_at=now.isoformat(),stale=False)
   events=[dict(type='delta',text=content),dict(type='done',summary=summary)]
   return self.send(''.join('data: '+json.dumps(event)+'\n\n' for event in events).encode(),mime='text/event-stream')
  if path.endswith('/chat/messages'):return self.send(dict(items=[]))
  if path.endswith('/chat'):
   reply='Коротко: actor-critic требует градиента log-вероятности действия, а у flow/diffusion-политик она задана неявно — через многошаговый процесс сэмплирования.\n\nАвторы используют критик на инференсе как направляющий градиент [p.1].'
   payload=dict(type='done',reply=reply,message_id=str(uuid.uuid4()))
   return self.send(('data: '+json.dumps(dict(type='delta',text=reply))+'\n\ndata: '+json.dumps(payload)+'\n\n').encode(),mime='text/event-stream')
  if path.endswith('/annotations'):
   if self.command=='POST':
    note=dict(**body,id=str(uuid.uuid4()),paper_id='paper-1',created_at=now.isoformat(),updated_at=now.isoformat());notes.append(note);return self.send(note)
   return self.send(notes)
  if path.startswith('/annotations/'):
   note=next(v for v in notes if v['id']==path.split('/')[-1])
   if self.command=='PATCH':note.update(body)
   if self.command=='DELETE':notes.remove(note)
   return self.send(note)
  if path.endswith('/pdf'):return self.send((ROOT/'src/shared/assets/qgf-flow-policies.pdf').read_bytes(),mime='application/pdf')
  if path.endswith('/pdf-url'):return self.send(dict(url='http://127.0.0.1:8089/papers/paper-1/pdf',expires_in=3600,status='ready',source='arxiv'))
  if path.endswith('/translate'):
   result=dict(translation='…но встроить их в RL-пайплайны для улучшения политики оказалось сложнее.',target_lang='ru')
   if query.get('stream')==['1']:
    data='data: '+json.dumps(dict(type='done',**result))+'\n\n'
    return self.send(data.encode(),mime='text/event-stream')
   return self.send(result)
  if path.startswith('/papers/'):
   return self.send(next((p for p in papers if p['id']==path.split('/')[-1] or p['arxiv_id']==body.get('arxiv_id')),papers[0]))
  self.send(dict(detail='Unknown fixture route: '+path),404)
print('In-memory v3 fixtures: http://127.0.0.1:8089',flush=True)
ThreadingHTTPServer(('127.0.0.1',8089),Handler).serve_forever()
