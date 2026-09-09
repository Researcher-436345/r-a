"""Compile the supplied static v3 component catalogue to native JSX + scoped CSS.
No DC runtime, injected HTML, or external scripts are used by the application.
Run from frontend: python3 scripts/import-design-system.py
"""
from html.parser import HTMLParser
from pathlib import Path
import json
import re

ROOT = Path(__file__).resolve().parents[1]
source = (ROOT / 'examples/v3/v3/Design System.dc.html').read_text()
source = source.split('</helmet>', 1)[1].split('</x-dc>', 1)[0]
def replace_cat(match):
    if 'catTail' not in match[0]: return match[0]
    opening_tag = match[0].split('>', 1)[0]
    variant = 'reader' if 'width="60"' in opening_tag else 'chat' if 'width="63"' in opening_tag else 'home'
    return f'<sleeping-cat variant="{variant}"></sleeping-cat>'
source = re.sub(r'<svg\b[^>]*>.*?</svg>', replace_cat, source, flags=re.S)
source = re.sub(r'<img\b[^>]*logo-ra.png[^>]*>\s*<span\b[^>]*>Odyssey</span>', '<logo-mark></logo-mark>', source)
COLORS = {'#fbfbfc':'var(--bg)', '#ffffff':'var(--panel)', '#191c20':'var(--text)', '#6a7178':'var(--sub)', '#8b9198':'var(--muted)', '#eceef1':'var(--border)', '#d9dde4':'var(--border-strong)', '#f5f6f8':'var(--hover)', '#f3f5fb':'var(--accent-soft)', '#233b92':'var(--accent-text)', '#1b2f78':'var(--accent-strong)'}
class Catalogue(HTMLParser):
    def __init__(self):
        super().__init__(); self.out=[]; self.styles={}; self.rules=[]
    def handle_starttag(self, tag, attrs):
        if tag in ['sleeping-cat', 'logo-mark']:
            self.out.append('<SleepingCat variant="'+dict(attrs).get('variant','home')+'" />' if tag=='sleeping-cat' else '<LogoMark />'); return
        attrs=dict(attrs); css=[]; result=[]
        for name in ['style','style-hover','style-focus']:
            value=attrs.pop(name,None)
            if value: css.append((name,value))
        if css:
            key=tuple(css)
            if key not in self.styles:
                cls='ds-s'+str(len(self.styles)+1); self.styles[key]=cls
                for name,value in css:
                    # Color swatches remain literal to represent the documented palette.
                    if 'height:28px' not in value and 'height:48px' not in value:
                        for old,new in COLORS.items(): value=value.replace(old,new)
                    pseudo={'style':'','style-hover':':hover','style-focus':':focus'}[name]
                    self.rules.append('.design-system .'+cls+pseudo+'{'+value+'}')
            result.append('className="'+self.styles[key]+'"')
        mapping={'viewbox':'viewBox','stroke-width':'strokeWidth','stroke-linecap':'strokeLinecap','stroke-linejoin':'strokeLinejoin','fill-rule':'fillRule','clip-rule':'clipRule','tabindex':'tabIndex','for':'htmlFor','class':'className'}
        for k,v in attrs.items():
            if k.startswith('on'):continue
            k=mapping.get(k,k)
            if k=='rows': result.append('rows={'+v+'}')
            elif v is None:result.append(k)
            else:result.append(k+'='+json.dumps(v,ensure_ascii=False))
        self.out.append('<'+tag+(' '+' '.join(result) if result else '')+(' />' if tag in ['input','img','br','hr'] else '>'))
    def handle_endtag(self, tag):
        if tag not in ['sleeping-cat','logo-mark','input','img','br','hr']: self.out.append('</'+tag+'>')
    def handle_data(self, data):
        if data.strip() == '{{ loader }}':
            self.out.append('<LoaderCircle className="ds-loader spin" size={17} aria-hidden="true" />'); return
        if data.strip(): self.out.append('{'+json.dumps(data,ensure_ascii=False)+'}')
    def handle_comment(self, data): pass
p=Catalogue();p.feed(source)
(ROOT/'src/pages/design-system/design-system-page.tsx').write_text('''// Compiled from Design System.dc.html by scripts/import-design-system.py.
import { LoaderCircle } from 'lucide-react';
import { LogoMark } from '../../shared/ui/logo-mark';
import { SleepingCat } from '../../shared/ui/sleeping-cat';
import './design-system.css';

export function DesignSystemPage() {
  return <div className="design-system">\n'''+ '\n'.join(p.out)+ '\n</div>;\n}\n')
(ROOT/'src/pages/design-system/design-system.css').write_text('/* Catalogue-only styles; application controls live in shared/styles/v3.css. */\n.design-system .ds-loader { color: var(--accent); flex: none; }\n.design-system .sleeping-cat { right: 20px; }\n'+'\n'.join(p.rules)+'\n')
