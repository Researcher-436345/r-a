// Compiled from Design System.dc.html by scripts/import-design-system.py.
import { LoaderCircle } from 'lucide-react';
import { LogoMark } from '../../shared/ui/logo-mark';
import { SleepingCat } from '../../shared/ui/sleeping-cat';
import './design-system.css';

export function DesignSystemPage() {
  return <div className="design-system">
<main className="ds-s1">
<div className="ds-s2">
<div className="ds-s3">
<LogoMark />
</div>
<h1 className="ds-s4">
{"Дизайн-система v3"}
</h1>
<p className="ds-s5">
{"Спокойный, плотный интерфейс для чтения и исследования. Один акцентный синий из логотипа, прямые углы у рабочих поверхностей, разделители вместо декора. Скругления — только у того, что «всплывает» над интерфейсом или призывает к действию: попапы, модалки, синие кнопки."}
</p>
<div className="ds-s6">
<h2 className="ds-s7">
{"Принципы"}
</h2>
</div>
<div className="ds-s8">
<div className="ds-s9">
<div className="ds-s10">
{"Два уровня скруглений"}
</div>
<p className="ds-s11">
{"Панели, списки, поля, карточки и вторичные кнопки — прямые углы. Всплывающее (попапы 16px, модалки 20px) и призывы к действию (синие кнопки — пилюли, отправка — круг) — скруглённые."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Один акцент"}
</div>
<p className="ds-s11">
{"Синий #233b92 — из логотипа. Активное состояние, ссылки, иконка действия, основная кнопка. Всё остальное — серая шкала."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Разделители, не точки"}
</div>
<p className="ds-s11">
{"Мета-строки разделяются линией 1×11px с отступом 12px. В тулбарах — микролинии 1×22px. Между строками списка — граница 1px."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Списки одним блоком"}
</div>
<p className="ds-s11">
{"Статьи, заметки, подсказки — один белый контейнер с разделителями, а не набор карточек. Ховер — подсветка строки #fbfbfc."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Селектор без двойного контура"}
</div>
<p className="ds-s11">
{"Выбранный сегмент — белая плашка во всю высоту, без тени и зазора. Заголовок панели может сам быть селектором (56px, во всю ширину)."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Fraunces — только дома"}
</div>
<p className="ds-s11">
{"Антиква на заголовке главной и на слове Odyssey. Заголовки остальных экранов — Golos Text 600, 22–24px."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Пустое состояние без рамок"}
</div>
<p className="ds-s11">
{"Подсказки и вводные тексты лежат прямо на панели: без карточек, серых фонов и иконок-заглушек."}
</p>
</div>
<div className="ds-s9">
<div className="ds-s10">
{"Кот — единственная иллюстрация"}
</div>
<p className="ds-s11">
{"Пиксельный спящий кот сидит на верхней кромке композера. Главная 84×60, ассистент 63×45, ридер 60×43."}
</p>
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Цвета"}
</h2>
</div>
<div className="ds-s12">
<div className="ds-s13">
<div className="ds-s14">
</div>
<div className="ds-s15">
{"Accent"}
</div>
<div className="ds-s16">
{"#233b92 · кнопки, активное, ссылки"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s17">
</div>
<div className="ds-s15">
{"Accent hover"}
</div>
<div className="ds-s16">
{"#1b2f78 · наведение"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s18">
</div>
<div className="ds-s15">
{"Accent soft"}
</div>
<div className="ds-s16">
{"#f3f5fb · активный фон"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s19">
</div>
<div className="ds-s15">
{"Accent muted"}
</div>
<div className="ds-s16">
{"#8d9ac9 · второстепенный акцент"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s20">
</div>
<div className="ds-s15">
{"Text"}
</div>
<div className="ds-s16">
{"#191c20 · основной текст"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s21">
</div>
<div className="ds-s15">
{"Muted"}
</div>
<div className="ds-s16">
{"#6a7178 · вторичный текст"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s22">
</div>
<div className="ds-s15">
{"Subtle"}
</div>
<div className="ds-s16">
{"#8b9198 · мета, подписи"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s23">
</div>
<div className="ds-s15">
{"Faint"}
</div>
<div className="ds-s16">
{"#a2a8af · плейсхолдеры"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s24">
</div>
<div className="ds-s15">
{"Background"}
</div>
<div className="ds-s16">
{"#fbfbfc · фон страницы"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s25">
</div>
<div className="ds-s15">
{"Surface"}
</div>
<div className="ds-s16">
{"#ffffff · панели, карточки"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s26">
</div>
<div className="ds-s15">
{"Hover"}
</div>
<div className="ds-s16">
{"#f5f6f8 · наведение на строки"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s27">
</div>
<div className="ds-s15">
{"Border"}
</div>
<div className="ds-s16">
{"#eceef1 · границы"}
</div>
</div>
<div className="ds-s13">
<div className="ds-s28">
</div>
<div className="ds-s15">
{"Border strong"}
</div>
<div className="ds-s16">
{"#d9dde4 · разделители, поля"}
</div>
</div>
</div>
<div className="ds-s29">
<div className="ds-s30">
{"Цвета выделений в PDF"}
</div>
<div className="ds-s31">
<div className="ds-s32">
</div>
<div className="ds-s33">
</div>
<div className="ds-s34">
</div>
<div className="ds-s35">
</div>
<div className="ds-s36">
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Типографика"}
</h2>
<p className="ds-s37">
{"Golos Text для интерфейса и чтения; Fraunces — только для логотипа и заголовка главной."}
</p>
</div>
<div className="ds-s38">
<div className="ds-s39">
<div className="ds-s16">
{"Display · Fraunces 600 · 30px"}
</div>
<div>
<div className="ds-s40">
{"Что исследуем сегодня?"}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Logo · Fraunces 700 · 21px"}
</div>
<div>
<div className="ds-s41">
{"Odyssey"}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Page title · Golos 600 · 24px"}
</div>
<div>
<div className="ds-s42">
{"Хочу прочитать"}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Heading · Golos 600 · 17.5px"}
</div>
<div>
<div className="ds-s43">
{"Test-Time Gradient Guidance of Flow Policies"}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Section · Golos 600 · 17.5px"}
</div>
<div>
<div className="ds-s44">
{"Трендовые статьи"}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Body · Golos 400 · 15px"}
</div>
<div>
<div className="ds-s45">
{"Индустрия уходит от классического PPO к более простым и стабильным схемам обучения."}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Body small · Golos 400 · 13.5px · muted"}
</div>
<div>
<div className="ds-s46">
{"Предлагаем метод управления flow-matching-политиками на этапе инференса."}
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"UI · Golos 500 · 14px / 13px"}
</div>
<div>
<div className="ds-s47">
<span className="ds-s48">
{"Исследовать"}
</span>
<span className="ds-s49">
{"Хочу прочитать"}
</span>
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Meta · Golos 400 · 12.5px · subtle"}
</div>
<div>
<div className="ds-s50">
<span>
{"8 сен 2026"}
</span>
<span className="ds-s51">
</span>
<span>
{"L. Chen, M. Okafor"}
</span>
</div>
</div>
</div>
<div className="ds-s39">
<div className="ds-s16">
{"Label · Golos 600 · 11.5px · caps"}
</div>
<div>
<div className="ds-s30">
{"Сегодня"}
</div>
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Кнопки"}
</h2>
</div>
<div className="ds-s52">
<div className="ds-s53">
<div className="ds-s30">
{"Primary · пилюля"}
</div>
<div className="ds-s54">
<div className="ds-s55">
<button className="ds-s56" type="submit">
{"Добавить в библиотеку"}
</button>
</div>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Send · круг"}
</div>
<div className="ds-s54">
<button className="ds-s57" type="submit" title="Отправить">
<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m5 12 7-7 7 7">
</path>
<path d="M12 19V5">
</path>
</svg>
</button>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Secondary"}
</div>
<div className="ds-s54">
<button className="ds-s58">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z">
</path>
</svg>
<span>
{"Хочу прочитать"}
</span>
</button>
<button className="ds-s59">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M15 3h6v6">
</path>
<path d="M10 14 21 3">
</path>
<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6">
</path>
</svg>
<span>
{"arXiv"}
</span>
</button>
<button className="ds-s58">
<span>
{"Назад"}
</span>
</button>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Ghost / text"}
</div>
<div className="ds-s54">
<button className="ds-s60">
<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<rect width="14" height="14" x="8" y="8" rx="0">
</rect>
<path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2">
</path>
</svg>
<span>
{"Копировать Markdown"}
</span>
</button>
<a className="ds-s61" href="#">
{"К библиотеке"}
</a>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Icon buttons"}
</div>
<div className="ds-s54">
<button className="ds-s62" title="Свернуть">
<svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<rect width="18" height="18" x="3" y="3" rx="0">
</rect>
<path d="M9 3v18">
</path>
<path d="m16 15-3-3 3-3">
</path>
</svg>
</button>
<button className="ds-s63" title="Удалить">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M3 6h18">
</path>
<path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6">
</path>
<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2">
</path>
<line x1="10" x2="10" y1="11" y2="17">
</line>
<line x1="14" x2="14" y1="11" y2="17">
</line>
</svg>
</button>
<button className="ds-s63" title="Закрыть">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M18 6 6 18">
</path>
<path d="m6 6 12 12">
</path>
</svg>
</button>
<button className="ds-s62" title="Скачать">
<svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4">
</path>
<polyline points="7 10 12 15 17 10">
</polyline>
<line x1="12" x2="12" y1="15" y2="3">
</line>
</svg>
</button>
<button className="ds-s64">
<svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48">
</path>
</svg>
</button>
<button className="ds-s65">
<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z">
</path>
<path d="m9 10 2 2 4-4">
</path>
</svg>
</button>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Chip / counter"}
</div>
<div className="ds-s54">
<div className="ds-s66">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M16 3a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2 1 1 0 0 1 1 1v1a2 2 0 0 1-2 2 1 1 0 0 0-1 1v2a1 1 0 0 0 1 1 6 6 0 0 0 6-6V5a2 2 0 0 0-2-2z">
</path>
<path d="M5 3a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2 1 1 0 0 1 1 1v1a2 2 0 0 1-2 2 1 1 0 0 0-1 1v2a1 1 0 0 0 1 1 6 6 0 0 0 6-6V5a2 2 0 0 0-2-2z">
</path>
</svg>
<span className="ds-s67">
{"12"}
</span>
</div>
<div className="ds-s68">
<span className="ds-s69">
<span className="ds-s70">
</span>
</span>
<span>
{"32%"}
</span>
</div>
<button className="ds-s71">
{"стр. 1 · Аннотация"}
</button>
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Сегментированный контроль"}
</h2>
</div>
<div className="ds-s52">
<div className="ds-s72">
<div className="ds-s30">
{"С иконками"}
</div>
<div className="ds-s54">
<div className="ds-s73">
<button className="ds-s74">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<circle cx="12" cy="12" r="10">
</circle>
<path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20">
</path>
<path d="M2 12h20">
</path>
</svg>
<span>
{"Веб-поиск"}
</span>
</button>
<button className="ds-s75">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m10.065 12.493-6.18 1.318a.934.934 0 0 1-1.108-.702l-.537-2.15a1.07 1.07 0 0 1 .691-1.265l13.504-4.44">
</path>
<path d="m13.56 11.747 4.332-.924">
</path>
<path d="m16 21-3.105-6.21">
</path>
<path d="M16.485 5.94a2 2 0 0 1 1.455-2.415l1.09-.272a1 1 0 0 1 1.212.727l1.515 6.06a1 1 0 0 1-.727 1.213l-1.09.272a2 2 0 0 1-2.425-1.455z">
</path>
<path d="m6.158 8.633 1.114 4.456">
</path>
<path d="m8 21 3.105-6.21">
</path>
<circle cx="12" cy="13" r="2">
</circle>
</svg>
<span>
{"Глубокое исследование"}
</span>
</button>
</div>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Текстовый"}
</div>
<div className="ds-s54">
<div className="ds-s73">
<button className="ds-s74">
<span>
{"arXiv"}
</span>
</button>
<button className="ds-s76">
<span>
{"DOI"}
</span>
</button>
<button className="ds-s75">
<span>
{"PDF"}
</span>
</button>
</div>
<div className="ds-s73">
<button className="ds-s74">
<span>
{"RU"}
</span>
</button>
<button className="ds-s75">
<span>
{"EN"}
</span>
</button>
</div>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Заголовок-селектор панели · 56px"}
</div>
<div className="ds-s77">
<button className="ds-s78">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z">
</path>
<path d="M20 3v4">
</path>
<path d="M22 5h-4">
</path>
</svg>
<span>
{"Ассистент"}
</span>
</button>
<button className="ds-s79">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M13.4 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-7.4">
</path>
<path d="M2 6h4">
</path>
<path d="M2 10h4">
</path>
<path d="M2 14h4">
</path>
<path d="M2 18h4">
</path>
<path d="M21.378 5.626a1 1 0 1 0-3.004-3.004l-5.01 5.012a2 2 0 0 0-.506.854l-.837 2.87a.5.5 0 0 0 .62.62l2.87-.837a2 2 0 0 0 .854-.506z">
</path>
</svg>
<span>
{"Заметки"}
</span>
</button>
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Поля ввода"}
</h2>
</div>
<div className="ds-s52">
<div className="ds-s53">
<div className="ds-s30">
{"Text field"}
</div>
<label className="ds-s80">
<span className="ds-s81">
{"arXiv ID или URL"}
</span>
<input className="ds-s82" type="text" placeholder="1706.03762 или https://arxiv.org/abs/1706.03762" />
</label>
</div>
<div className="ds-s83">
<div className="ds-s30">
{"Composer"}
</div>
<form className="ds-s84">
<textarea className="ds-s85" rows={2} placeholder="Спросите о статьях, идеях или направлениях исследований…">
</textarea>
<div className="ds-s86">
<button className="ds-s87" title="Прикрепить файл">
<svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48">
</path>
</svg>
</button>
<div className="ds-s73">
<button className="ds-s74">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<circle cx="12" cy="12" r="10">
</circle>
<path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20">
</path>
<path d="M2 12h20">
</path>
</svg>
<span>
{"Веб-поиск"}
</span>
</button>
<button className="ds-s75">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m10.065 12.493-6.18 1.318a.934.934 0 0 1-1.108-.702l-.537-2.15a1.07 1.07 0 0 1 .691-1.265l13.504-4.44">
</path>
<path d="m13.56 11.747 4.332-.924">
</path>
<path d="m16 21-3.105-6.21">
</path>
<path d="M16.485 5.94a2 2 0 0 1 1.455-2.415l1.09-.272a1 1 0 0 1 1.212.727l1.515 6.06a1 1 0 0 1-.727 1.213l-1.09.272a2 2 0 0 1-2.425-1.455z">
</path>
<path d="m6.158 8.633 1.114 4.456">
</path>
<path d="m8 21 3.105-6.21">
</path>
<circle cx="12" cy="13" r="2">
</circle>
</svg>
<span>
{"Глубокое исследование"}
</span>
</button>
</div>
<div className="ds-s88">
</div>
<button className="ds-s57" type="submit" title="Отправить">
<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m5 12 7-7 7 7">
</path>
<path d="M12 19V5">
</path>
</svg>
</button>
</div>
</form>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Навигация"}
</h2>
</div>
<div className="ds-s52">
<div className="ds-s53">
<div className="ds-s30">
{"Пункт сайдбара · default / active"}
</div>
<div className="ds-s89">
<a className="ds-s90" href="#">
<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<circle cx="12" cy="12" r="10">
</circle>
<path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20">
</path>
<path d="M2 12h20">
</path>
</svg>
<span>
{"Исследовать"}
</span>
</a>
<a className="ds-s91" href="#">
<svg className="ds-s92" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m10.065 12.493-6.18 1.318a.934.934 0 0 1-1.108-.702l-.537-2.15a1.07 1.07 0 0 1 .691-1.265l13.504-4.44">
</path>
<path d="m13.56 11.747 4.332-.924">
</path>
<path d="m16 21-3.105-6.21">
</path>
<path d="M16.485 5.94a2 2 0 0 1 1.455-2.415l1.09-.272a1 1 0 0 1 1.212.727l1.515 6.06a1 1 0 0 1-.727 1.213l-1.09.272a2 2 0 0 1-2.425-1.455z">
</path>
<path d="m6.158 8.633 1.114 4.456">
</path>
<path d="m8 21 3.105-6.21">
</path>
<circle cx="12" cy="13" r="2">
</circle>
</svg>
<span>
{"Ассистент"}
</span>
</a>
<a className="ds-s91" href="#">
<svg className="ds-s92" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M10 2v8l3-3 3 3V2">
</path>
<path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H19a1 1 0 0 1 1 1v18a1 1 0 0 1-1 1H6.5a1 1 0 0 1 0-5H20">
</path>
</svg>
<span>
{"Библиотека"}
</span>
</a>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Дерево папок"}
</div>
<div className="ds-s93">
<div className="ds-s94">
<button className="ds-s95" title="Хочу прочитать">
<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M12 7v14">
</path>
<path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z">
</path>
</svg>
<span className="ds-s96">
{"Хочу прочитать"}
</span>
</button>
<span className="ds-s97">
{"12"}
</span>
</div>
<div className="ds-s98">
<button className="ds-s99" title="Читаю">
<svg className="ds-s92" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<circle cx="12" cy="12" r="10">
</circle>
<polyline points="12 6 12 12 16.5 12">
</polyline>
</svg>
<span className="ds-s96">
{"Читаю"}
</span>
</button>
<span className="ds-s97">
{"3"}
</span>
</div>
<div className="ds-s98">
<button className="ds-s99" title="RL / Flow policies">
<svg className="ds-s92" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z">
</path>
</svg>
<span className="ds-s96">
{"RL / Flow policies"}
</span>
</button>
<button className="ds-s100">
<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m6 9 6 6 6-6">
</path>
</svg>
</button>
<span className="ds-s97">
{"5"}
</span>
</div>
<div className="ds-s101">
<button className="ds-s99" title="Test-time compute">
<svg className="ds-s92" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z">
</path>
</svg>
<span className="ds-s96">
{"Test-time compute"}
</span>
</button>
<span className="ds-s97">
{"2"}
</span>
</div>
</div>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"История чатов"}
</div>
<div className="ds-s93">
<div className="ds-s30">
{"Сегодня"}
</div>
<div className="ds-s102">
<button className="ds-s103">
{"Производительный веб-поиск"}
</button>
<button className="ds-s104" title="Удалить диалог">
<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M3 6h18">
</path>
<path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6">
</path>
<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2">
</path>
<line x1="10" x2="10" y1="11" y2="17">
</line>
<line x1="14" x2="14" y1="11" y2="17">
</line>
</svg>
</button>
</div>
<div className="ds-s105">
<button className="ds-s106">
{"Свежие методы RLHF: обзор"}
</button>
<button className="ds-s104" title="Удалить диалог">
<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M3 6h18">
</path>
<path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6">
</path>
<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2">
</path>
<line x1="10" x2="10" y1="11" y2="17">
</line>
<line x1="14" x2="14" y1="11" y2="17">
</line>
</svg>
</button>
</div>
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Карточки и списки"}
</h2>
<p className="ds-s37">
{"Списки — один белый блок с разделителями между строками, без отдельных карточек."}
</p>
</div>
<div className="ds-s107">
<div className="ds-s38">
<article className="ds-s108">
<div className="ds-s109">
<a className="ds-s110" href="#">
{"Sparse Retrieval Heads Explain Long-Context Failures in Transformers"}
</a>
<div className="ds-s111">
<span className="ds-s112">
{"arXiv:2609.01377"}
</span>
<button className="ds-s63" title="Удалить из библиотеки">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M3 6h18">
</path>
<path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6">
</path>
<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2">
</path>
<line x1="10" x2="10" y1="11" y2="17">
</line>
<line x1="14" x2="14" y1="11" y2="17">
</line>
</svg>
</button>
</div>
</div>
<div className="ds-s113">
{"A. Petrov, S. Nakamura"}
</div>
<p className="ds-s114">
{"Мы выделяем небольшой набор голов внимания, отвечающих за копирование из длинного контекста, и показываем, что их насыщение предсказывает сбои извлечения."}
</p>
</article>
</div>
<div className="ds-s52">
<div className="ds-s53">
<div className="ds-s30">
{"Мини-обложка"}
</div>
<button className="ds-s115">
<div className="ds-s116">
{"2026"}
</div>
<div className="ds-s117">
{"Test-Time Gradient Guidance of Flow Policies in Reinforcement Learning"}
</div>
<div className="ds-s118">
{"Предлагаем метод управления flow-matching-политиками на этапе инференса с помощью градиентов обученной функции ценности, что повышает эффективность выборки без переобучения базовой политики."}
</div>
<div className="ds-s119">
{"arXiv:2609.01842"}
</div>
</button>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Заметка · строка списка"}
</div>
<article className="ds-s120">
<div className="ds-s121">
<svg className="ds-s122" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M16 3a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2 1 1 0 0 1 1 1v1a2 2 0 0 1-2 2 1 1 0 0 0-1 1v2a1 1 0 0 0 1 1 6 6 0 0 0 6-6V5a2 2 0 0 0-2-2z">
</path>
<path d="M5 3a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2 1 1 0 0 1 1 1v1a2 2 0 0 1-2 2 1 1 0 0 0-1 1v2a1 1 0 0 0 1 1 6 6 0 0 0 6-6V5a2 2 0 0 0-2-2z">
</path>
</svg>
<span>
{"…incorporating them into RL pipelines for policy improvement has proven more difficult."}
</span>
</div>
<p className="ds-s123">
{"Ключевая мотивация: flow/diffusion-политики плохо встраиваются в RL из-за нестабильности обучения актора."}
</p>
<div className="ds-s124">
<span>
{"стр. 1"}
</span>
<span className="ds-s88">
</span>
<button className="ds-s125" title="Редактировать">
<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z">
</path>
<path d="m15 5 4 4">
</path>
</svg>
</button>
<button className="ds-s125" title="Удалить">
<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M3 6h18">
</path>
<path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6">
</path>
<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2">
</path>
<line x1="10" x2="10" y1="11" y2="17">
</line>
<line x1="14" x2="14" y1="11" y2="17">
</line>
</svg>
</button>
</div>
</article>
</div>
<div className="ds-s53">
<div className="ds-s30">
{"Подсказки · empty state"}
</div>
<div className="ds-s38">
<button className="ds-s126">
<svg className="ds-s127" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<polyline points="15 10 20 15 15 20">
</polyline>
<path d="M4 4v7a4 4 0 0 0 4 4h12">
</path>
</svg>
<span>
{"Объясни основную идею простыми словами"}
</span>
</button>
<button className="ds-s128">
<svg className="ds-s127" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<polyline points="15 10 20 15 15 20">
</polyline>
<path d="M4 4v7a4 4 0 0 0 4 4h12">
</path>
</svg>
<span>
{"Какие основные результаты получили авторы?"}
</span>
</button>
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Тулбар ридера"}
</h2>
<p className="ds-s37">
{"Группы действий разделены микролиниями 1×22px; зум — «−  100%  +» без рамки; значения серые."}
</p>
</div>
<div className="ds-s107">
<div className="ds-s129">
<button className="ds-s130" title="Добавить в папку">
<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z">
</path>
<path d="m9 10 2 2 4-4">
</path>
</svg>
</button>
<span className="ds-s131">
</span>
<div className="ds-s132">
<div className="ds-s133">
{"Test-Time Gradient Guidance of Flow Policies"}
</div>
<div className="ds-s134">
<span>
{"Zhou et al."}
</span>
<span className="ds-s135">
</span>
<span>
{"arXiv:2606.11087"}
</span>
</div>
</div>
<div className="ds-s136">
<button className="ds-s63" title="Уменьшить">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M5 12h14">
</path>
</svg>
</button>
<button className="ds-s137">
{"100%"}
</button>
<button className="ds-s63" title="Увеличить">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M5 12h14">
</path>
<path d="M12 5v14">
</path>
</svg>
</button>
</div>
<span className="ds-s138">
</span>
<div className="ds-s139">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z">
</path>
<path d="M14 2v4a2 2 0 0 0 2 2h4">
</path>
<path d="M10 9H8">
</path>
<path d="M16 13H8">
</path>
<path d="M16 17H8">
</path>
</svg>
<span className="ds-s67">
{"1 / 14"}
</span>
</div>
<span className="ds-s138">
</span>
<button className="ds-s62" title="Скачать PDF">
<svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4">
</path>
<polyline points="7 10 12 15 17 10">
</polyline>
<line x1="12" x2="12" y1="15" y2="3">
</line>
</svg>
</button>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Иллюстрация · кот"}
</h2>
<p className="ds-s37">
{"Пиксельная сетка 4px, два тона синего. Сидит на верхней кромке композера справа, лапы свисают внутрь. Анимации: дыхание 3.2s, хвост 1.6s, «z» 2.4s — все ступенчатые (steps)."}
</p>
</div>
<div className="ds-s52">
<div className="ds-s83">
<div className="ds-s30">
{"Главная · 84×60"}
</div>
<div className="ds-s140">
<SleepingCat variant="home" />
</div>
</div>
<div className="ds-s83">
<div className="ds-s30">
{"Ассистент · 63×45"}
</div>
<div className="ds-s140">
<SleepingCat variant="chat" />
</div>
</div>
<div className="ds-s83">
<div className="ds-s30">
{"Ридер · 60×43"}
</div>
<div className="ds-s141">
<SleepingCat variant="reader" />
</div>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Сообщения чата"}
</h2>
</div>
<div className="ds-s142">
<article className="ds-s143">
<p className="ds-s144">
{"Собери обзор по свежим методам RLHF"}
</p>
</article>
<article className="ds-s145">
<p className="ds-s144">
{"Ответ ассистента — обычный текст без пузыря. Источники — "}
<a className="ds-s146" href="#">
{"подчёркнутые ссылки"}
</a>
{" акцентного цвета."}
</p>
</article>
<div className="ds-s147">
<LoaderCircle className="ds-loader spin" size={17} aria-hidden="true" />
<span>
{"Читаю первоисточники…"}
</span>
</div>
</div>
<div className="ds-s6">
<h2 className="ds-s7">
{"Всплывающие элементы"}
</h2>
<p className="ds-s148">
{"Единственные скруглённые контейнеры: попапы и меню 16px, модалка настроек 20px, тост — пилюля."}
</p>
</div>
<div className="ds-s149">
<div className="ds-s83">
<div className="ds-s30">
{"Действие с выделением"}
</div>
<div className="ds-s150">
<div className="ds-s151" role="dialog">
<div className="ds-s152">
<button className="ds-s153" title="Цвет выделения">
</button>
<button className="ds-s154" title="Цвет выделения">
</button>
<button className="ds-s155" title="Цвет выделения">
</button>
<button className="ds-s156" title="Цвет выделения">
</button>
<button className="ds-s157" title="Цвет выделения">
</button>
</div>
<div className="ds-s158">
<button className="ds-s159">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z">
</path>
<path d="M13 8H7">
</path>
<path d="M17 12H7">
</path>
</svg>
<span>
{"Спросить"}
</span>
</button>
<button className="ds-s159">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M13.4 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-7.4">
</path>
<path d="M2 6h4">
</path>
<path d="M2 10h4">
</path>
<path d="M2 14h4">
</path>
<path d="M2 18h4">
</path>
<path d="M21.378 5.626a1 1 0 1 0-3.004-3.004l-5.01 5.012a2 2 0 0 0-.506.854l-.837 2.87a.5.5 0 0 0 .62.62l2.87-.837a2 2 0 0 0 .854-.506z">
</path>
</svg>
<span>
{"Заметка"}
</span>
</button>
<span className="ds-s88">
</span>
<button className="ds-s63" title="Закрыть">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M18 6 6 18">
</path>
<path d="m6 6 12 12">
</path>
</svg>
</button>
</div>
<div className="ds-s160">
{"…но встроить их в RL-пайплайны для улучшения политики оказалось сложнее."}
</div>
</div>
</div>
</div>
<div className="ds-s83">
<div className="ds-s30">
{"Меню папок"}
</div>
<div className="ds-s161">
<div className="ds-s162">
{"Добавить в папку"}
</div>
<button className="ds-s163">
<svg className="ds-s92" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M12 7v14">
</path>
<path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z">
</path>
</svg>
<span className="ds-s88">
{"Хочу прочитать"}
</span>
<svg className="ds-s164" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M20 6 9 17l-5-5">
</path>
</svg>
</button>
<button className="ds-s163">
<svg className="ds-s92" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<circle cx="12" cy="12" r="10">
</circle>
<polyline points="12 6 12 12 16.5 12">
</polyline>
</svg>
<span className="ds-s88">
{"Читаю"}
</span>
</button>
<button className="ds-s163">
<svg className="ds-s92" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<rect width="20" height="5" x="2" y="3" rx="0">
</rect>
<path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8">
</path>
<path d="M10 12h4">
</path>
</svg>
<span className="ds-s88">
{"Другое"}
</span>
</button>
<button className="ds-s163">
<svg className="ds-s92" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z">
</path>
</svg>
<span className="ds-s88">
{"RL / Flow policies"}
</span>
</button>
</div>
</div>
<div className="ds-s83">
<div className="ds-s30">
{"Тост"}
</div>
<div className="ds-s165">
<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
<path d="M20 6 9 17l-5-5">
</path>
</svg>
<span className="ds-s166">
{"Статья добавлена в папку"}
</span>
</div>
</div>
</div>
</div>
</main>
</div>;
}
