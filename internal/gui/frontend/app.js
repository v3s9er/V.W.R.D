const $ = id => document.getElementById(id);
const svgNS = 'http://www.w3.org/2000/svg';
const stateNames = {idle:'대기 중',starting:'준비 중',running:'수집 중',complete:'완료',stopped:'중지됨',error:'오류'};
const minGraphZoom = .002;
const maxGraphZoom = 4;
let bridge;
let polling = false;
let toastTimer;
let lastReportDir = '';
let resultDir = '';
let graphReturnFocus = null;
let graph = {menus:[], pages:new Map(), elements:new Map(), width:0, height:0, zoom:1};

function showToast(message, error = false) {
  const toast = $('toast');
  toast.textContent = message;
  toast.className = 'toast show' + (error ? ' error' : '');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.className = 'toast', 4200);
}

function renderStatus(status) {
  const busy = status.state === 'running' || status.state === 'starting';
  const pill = $('status-pill');
  pill.textContent = stateNames[status.state] || status.state || '대기 중';
  pill.className = 'status-pill' + (busy ? ' running' : status.state === 'error' ? ' error' : '');
  $('status-message').textContent = status.message || '준비되었습니다';
  $('progress-count').textContent = `${status.visited || 0} / ${status.discovered || 0}`;
  $('discovered-count').textContent = (status.discovered || 0).toLocaleString('ko-KR');
  $('saved-count').textContent = (status.saved || 0).toLocaleString('ko-KR');
  $('current-url').textContent = status.current_url || '—';
  $('current-url').title = status.current_url || '';
  resultDir = lastReportDir || status.result_dir || '';
  $('result-path').textContent = resultDir || '—';
  $('result-path').title = resultDir;
  const percent = status.state === 'complete' ? 100 : status.discovered ? Math.min(98, Math.round((status.visited || 0) / status.discovered * 100)) : 0;
  $('progress-fill').style.width = `${percent}%`;
  $('start').disabled = busy;
  $('stop').disabled = !busy;
  $('open-browser').disabled = busy;
  $('open-result').disabled = !resultDir;
  $('graph-btn').disabled = busy;
  $('load-report').disabled = busy;
}

async function refreshStatus() {
  if (!bridge || polling) return;
  polling = true;
  try { renderStatus(await bridge.Status()); }
  catch (error) { showToast(String(error), true); }
  finally { polling = false; }
}

function numberValue(id, fallback, minimum, maximum = Number.MAX_SAFE_INTEGER) {
  const value = Number.parseInt($(id).value, 10);
  return Number.isFinite(value) ? Math.min(maximum, Math.max(minimum, value)) : fallback;
}

function address() {
  try {
    const url = new URL($('site-url').value.trim());
    if (url.protocol === 'http:' || url.protocol === 'https:') return url.href;
  } catch {}
  $('site-url').focus();
  showToast('HTTP 또는 HTTPS 주소를 입력하세요.', true);
  return null;
}

async function waitForBridge() {
  for (let i = 0; i < 100; i++) {
    if (window.go?.gui?.Bridge) return window.go.gui.Bridge;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error('앱 연결에 실패했습니다. 다시 실행해 주세요.');
}

async function initialise() {
  try {
    bridge = await waitForBridge();
    const defaults = await bridge.Defaults();
    $('output-dir').value = defaults.output_dir || '';
    $('min-delay').value = defaults.min_delay_ms || 1000;
    $('idle-time').value = defaults.idle_ms || 3000;
    $('max-wait').value = defaults.max_wait_ms || 45000;
    $('max-menus').value = defaults.max_menus || 1000;
    await refreshStatus();
    setInterval(refreshStatus, 800);
  } catch (error) { showToast(String(error), true); }
}

$('choose-dir').addEventListener('click', async () => {
  if (!bridge) return;
  try { const dir = await bridge.ChooseDirectory($('output-dir').value.trim()); if (dir) $('output-dir').value = dir; }
  catch (error) { showToast(String(error), true); }
});
$('open-browser').addEventListener('click', async () => {
  const url = address();
  if (!url || !bridge) return;
  $('open-browser').disabled = true;
  try { await bridge.OpenBrowser(url); await refreshStatus(); }
  catch (error) { showToast(String(error), true); }
  finally { $('open-browser').disabled = false; }
});
$('start').addEventListener('click', async () => {
  const url = address();
  if (!url || !bridge) return;
  const output = $('output-dir').value.trim();
  if (!output) { $('output-dir').focus(); showToast('저장 폴더를 입력하세요.', true); return; }
  $('start').disabled = true;
  try {
    lastReportDir = '';
    await bridge.Start({
      url, output_dir:output, menu_selector:$('menu-selector').value.trim(),
      min_delay_ms:numberValue('min-delay',1000,1000), idle_ms:numberValue('idle-time',3000,1000),
      max_wait_ms:numberValue('max-wait',45000,2000), max_menus:numberValue('max-menus',1000,1,1000)
    });
    await refreshStatus();
  } catch (error) { showToast(String(error), true); await refreshStatus(); }
});
$('stop').addEventListener('click', async () => {
  if (!bridge) return;
  try { await bridge.Stop(); await refreshStatus(); }
  catch (error) { showToast(String(error), true); }
});
$('open-result').addEventListener('click', async () => {
  if (!bridge) return;
  try { await bridge.OpenResult(resultDir); }
  catch (error) { showToast(String(error), true); }
});

function svgElement(name, attrs = {}, text = '') {
  const element = document.createElementNS(svgNS, name);
  for (const [key, value] of Object.entries(attrs)) element.setAttribute(key, value);
  if (text) element.textContent = text;
  return element;
}
function shortLabel(value, maximum = 19) {
  const runes = Array.from(value || '메뉴');
  return runes.length > maximum ? runes.slice(0, maximum - 1).join('') + '…' : runes.join('');
}

function pageResources(page) {
  if (!page) return [];
  const resources = [...(page.resources || [])];
  if (page.html_path && !resources.some(item => item.path === page.html_path)) {
    resources.push({path:page.html_path,url:page.final_url || page.menu.url});
  }
  return resources;
}

function selectNode(node) {
  for (const element of graph.elements.values()) element.classList.remove('selected');
  graph.elements.get(node.id)?.classList.add('selected');
  const menu = node.menu;
  const page = menu ? graph.pages.get(menu.url) : null;
  const resources = pageResources(page);
  $('graph-detail-empty').hidden = true;
  $('graph-detail').hidden = false;
  $('detail-title').textContent = node.label;
  $('detail-url').textContent = menu ? menu.url : node.path.join(' › ');
  $('detail-state').textContent = node.group ? '메뉴 분류' : page ? (page.wait_result === 'timeout' ? '대기 시간 초과' : '방문 완료') : '방문 전';
  $('detail-count').textContent = node.group ? `${node.childCount}개 하위 항목` : `${resources.length}개 파일`;
  const list = $('detail-resources');
  list.replaceChildren();
  if (!resources.length) {
    const empty = document.createElement('div'); empty.className = 'graph-empty'; empty.textContent = node.group ? '하위 메뉴를 선택하세요.' : '기록된 리소스가 없습니다.'; list.append(empty); return;
  }
  const fragment = document.createDocumentFragment();
  for (const resource of resources) {
    const row = document.createElement('div'); row.className = 'resource-row' + (resource.error ? ' error' : '');
    const path = document.createElement('strong'); path.textContent = resource.path || resource.url;
    const url = document.createElement('small'); url.textContent = resource.error || resource.url;
    row.append(path, url); fragment.append(row);
  }
  list.append(fragment);
}

function setZoom(value, focus = null, preserveFocus = true) {
  if (!graph.width || !graph.height) return;
  const svg = $('graph-svg');
  const canvas = $('graph-canvas');
  const oldRect = svg.getBoundingClientRect();
  const canvasRect = canvas.getBoundingClientRect();
  let point = focus || {clientX:canvasRect.left + canvasRect.width / 2, clientY:canvasRect.top + canvasRect.height / 2};
  if (focus && (point.clientX < oldRect.left || point.clientX > oldRect.right || point.clientY < oldRect.top || point.clientY > oldRect.bottom)) {
    point = {clientX:canvasRect.left + canvasRect.width / 2, clientY:canvasRect.top + canvasRect.height / 2};
  }
  const oldZoom = graph.zoom || 1;
  const localX = (point.clientX - oldRect.left) / oldZoom;
  const localY = (point.clientY - oldRect.top) / oldZoom;
  graph.zoom = Math.max(minGraphZoom, Math.min(maxGraphZoom, value));
  svg.style.width = `${Math.max(1, graph.width * graph.zoom)}px`;
  svg.style.height = `${Math.max(1, graph.height * graph.zoom)}px`;
  const percent = graph.zoom * 100;
  $('graph-zoom').textContent = `${percent < 1 ? Number(percent.toFixed(2)) : percent < 10 ? Number(percent.toFixed(1)) : Math.round(percent)}%`;
  $('graph-minus').disabled = graph.zoom <= minGraphZoom;
  $('graph-plus').disabled = graph.zoom >= maxGraphZoom;
  if (preserveFocus && oldRect.width && oldRect.height) {
    const nextRect = svg.getBoundingClientRect();
    canvas.scrollLeft += nextRect.left + localX * graph.zoom - point.clientX;
    canvas.scrollTop += nextRect.top + localY * graph.zoom - point.clientY;
  }
}

function fitGraph() {
  if (!graph.width || !graph.height) return;
  const canvas = $('graph-canvas');
  setZoom(Math.min(1, (canvas.clientWidth - 32) / graph.width, (canvas.clientHeight - 32) / graph.height), null, false);
  canvas.scrollTo(0, 0);
}

function renderGraph(report) {
  const menus = report.menus?.length ? report.menus : (report.pages || []).map(page => page.menu);
  graph = {menus, pages:new Map((report.pages || []).map(page => [page.menu.url, page])), nodes:new Map(), elements:new Map(), width:0, height:0, zoom:1};
  const svg = $('graph-svg'); svg.replaceChildren();
  const byURL = new Map(menus.map(menu => [menu.url, menu]));
  const rootMenu = byURL.get(report.root_url) || menus[0];
  menus.forEach((menu, orderIndex) => graph.nodes.set(menu.url, {id:menu.url, label:menu.label || menu.url, menu, parentId:null, group:false, orderIndex}));
  for (const [orderIndex, menu] of menus.entries()) {
    if (menu === rootMenu) continue;
    const node = graph.nodes.get(menu.url);
    const baseParent = menu.parent_url && byURL.has(menu.parent_url) && menu.parent_url !== menu.url ? menu.parent_url : rootMenu?.url;
    const path = Array.isArray(menu.path) && menu.path.length ? menu.path.filter(Boolean) : [menu.label || menu.url];
    const ancestors = path[0] === rootMenu?.label ? path.slice(1, -1) : path.slice(0, -1);
    let parentId = baseParent || null;
    const prefix = [];
    for (const label of ancestors) {
      prefix.push(label);
      const id = `group:${baseParent || 'root'}:${prefix.join(' › ')}`;
      if (!graph.nodes.has(id)) graph.nodes.set(id, {id, label, path:[...prefix], parentId, group:true, childCount:0, orderIndex});
      parentId = id;
    }
    node.parentId = parentId;
  }
  const children = new Map([...graph.nodes.keys()].map(id => [id, []]));
  const roots = [];
  for (const node of graph.nodes.values()) {
    if (node.parentId && children.has(node.parentId)) children.get(node.parentId).push(node);
    else roots.push(node);
  }
  for (const siblings of children.values()) siblings.sort((a, b) => a.orderIndex - b.orderIndex);
  roots.sort((a, b) => a.orderIndex - b.orderIndex);
  for (const node of graph.nodes.values()) if (node.group) node.childCount = children.get(node.id).length;
  const placements = new Map();
  const visited = new Set();
  let column = 0, maxDepth = 0;
  function place(node, depth) {
    if (visited.has(node.id)) return placements.get(node.id);
    visited.add(node.id); maxDepth = Math.max(maxDepth, depth);
    const descendants = (children.get(node.id) || []).filter(child => !visited.has(child.id));
    let x;
    if (!descendants.length) x = 44 + column++ * 208;
    else {
      const xs = descendants.map(child => place(child, depth + 1).x);
      x = (xs[0] + xs[xs.length - 1]) / 2;
    }
    const point = {x, y:46 + depth * 112}; placements.set(node.id, point); return point;
  }
  roots.forEach(node => place(node, 0));
  [...graph.nodes.values()].filter(node => !visited.has(node.id)).forEach(node => place(node, 0));
  graph.width = Math.max(1, column) * 208 + 80;
  graph.height = (maxDepth + 1) * 112 + 80;
  graph.placements = placements;
  svg.setAttribute('viewBox', `0 0 ${graph.width} ${graph.height}`);

  const edges = svgElement('g');
  for (const node of graph.nodes.values()) {
    const parent = placements.get(node.parentId), point = placements.get(node.id);
    if (!parent || !point || node.parentId === node.id) continue;
    const x1 = parent.x + 89, x2 = point.x + 89, y1 = parent.y + 52, y2 = point.y, mid = (y1 + y2) / 2;
    edges.append(svgElement('path', {class:'graph-edge',d:`M ${x1} ${y1} C ${x1} ${mid}, ${x2} ${mid}, ${x2} ${y2}`}));
  }
  svg.append(edges);
  for (const node of graph.nodes.values()) {
    const point = placements.get(node.id), page = node.menu ? graph.pages.get(node.menu.url) : null;
    const state = node.group ? '메뉴 분류' : page ? '방문 완료' : '방문 전';
    const group = svgElement('g', {class:'graph-node' + (node.group ? ' group' : page ? '' : ' unvisited'),transform:`translate(${point.x} ${point.y})`,tabindex:'0',role:'button','aria-label':`${node.label}, ${state}`});
    group.append(svgElement('rect',{x:0,y:0,width:178,height:52,rx:10}));
    group.append(svgElement('circle',{class:'node-dot',cx:15,cy:18,r:4}));
    group.append(svgElement('text',{class:'node-label',x:25,y:22},shortLabel(node.label)));
    group.append(svgElement('text',{class:'node-meta',x:15,y:40},node.group ? `${node.childCount}개 하위 항목` : page ? `${pageResources(page).length}개 파일` : '방문 전'));
    group.append(svgElement('title',{},node.menu?.url || node.path.join(' › ')));
    group.addEventListener('click',() => selectNode(node));
    group.addEventListener('keydown',event => {if(event.key==='Enter'||event.key===' '){event.preventDefault();selectNode(node);}});
    graph.elements.set(node.id, group); svg.append(group);
  }
  const saved = new Set((report.pages || []).flatMap(page => pageResources(page).map(resource => resource.path).filter(Boolean))).size;
  $('graph-summary').textContent = `발견 ${menus.length}개 메뉴 · 방문 ${report.pages?.length || 0}개 · 저장 파일 ${saved}개`;
  $('graph-canvas').scrollTo(0, 0);
  setZoom(1, null, false);
  if (rootMenu) selectNode(graph.nodes.get(rootMenu.url));
}

async function openGraph(directory = '') {
  if (!bridge) return;
  try {
    const selected = directory || resultDir || $('output-dir').value.trim();
    const report = await bridge.OpenReport(selected);
    if (!report) return;
    renderGraph(report);
    lastReportDir = report.output_dir || selected;
    await refreshStatus();
    graphReturnFocus = document.activeElement;
    $('graph-overlay').hidden = false;
    const root = graph.nodes.get(report.root_url) || graph.nodes.values().next().value;
    const origin = root && graph.placements.get(root.id);
    if (origin) {
      const canvas = $('graph-canvas');
      canvas.scrollLeft = origin.x * graph.zoom + 89 * graph.zoom - canvas.clientWidth / 2;
      canvas.scrollTop = 0;
    }
    $('graph-close').focus();
  } catch (error) { showToast(String(error), true); }
}
function closeGraph() { $('graph-overlay').hidden = true; (graphReturnFocus?.isConnected ? graphReturnFocus : $('graph-btn')).focus(); }
$('graph-btn').addEventListener('click', () => openGraph());
$('load-report').addEventListener('click', async () => {
  if (!bridge) return;
  try {
    const directory = await bridge.ChooseReportDirectory(lastReportDir || $('output-dir').value.trim());
    if (directory) await openGraph(directory);
  } catch (error) { showToast(String(error), true); }
});
$('graph-close').addEventListener('click', closeGraph);
$('graph-overlay').addEventListener('click', event => { if (event.target === $('graph-overlay')) closeGraph(); });
document.addEventListener('keydown', event => {
  if ($('graph-overlay').hidden) return;
  if (event.key === 'Escape') { closeGraph(); return; }
  if (event.target?.closest?.('input, textarea')) return;
  if (event.key === '+' || event.key === '=') { event.preventDefault(); setZoom(graph.zoom * 1.25); }
  if (event.key === '-') { event.preventDefault(); setZoom(graph.zoom / 1.25); }
  if (event.key === '0') { event.preventDefault(); fitGraph(); }
});
$('graph-plus').addEventListener('click', () => setZoom(graph.zoom * 1.25));
$('graph-minus').addEventListener('click', () => setZoom(graph.zoom / 1.25));
$('graph-fit').addEventListener('click', fitGraph);
const graphCanvas = $('graph-canvas');
graphCanvas.addEventListener('wheel', event => {
  if ($('graph-overlay').hidden || !event.deltaY) return;
  event.preventDefault();
  setZoom(graph.zoom * (event.deltaY < 0 ? 1.2 : 1 / 1.2), event);
}, {passive:false});
let graphPan = null;
graphCanvas.addEventListener('pointerdown', event => {
  if (event.pointerType !== 'mouse' || event.button !== 0 || event.target?.closest?.('.graph-node')) return;
  graphPan = {id:event.pointerId, x:event.clientX, y:event.clientY, left:graphCanvas.scrollLeft, top:graphCanvas.scrollTop};
  graphCanvas.setPointerCapture(event.pointerId);
  graphCanvas.classList.add('is-panning');
});
graphCanvas.addEventListener('pointermove', event => {
  if (!graphPan || event.pointerId !== graphPan.id) return;
  graphCanvas.scrollLeft = graphPan.left - (event.clientX - graphPan.x);
  graphCanvas.scrollTop = graphPan.top - (event.clientY - graphPan.y);
});
function endGraphPan(event) {
  if (!graphPan || event.pointerId !== graphPan.id) return;
  graphPan = null;
  graphCanvas.classList.remove('is-panning');
  if (graphCanvas.hasPointerCapture(event.pointerId)) graphCanvas.releasePointerCapture(event.pointerId);
}
graphCanvas.addEventListener('pointerup', endGraphPan);
graphCanvas.addEventListener('pointercancel', endGraphPan);
$('graph-search').addEventListener('input', event => {
  const query = event.target.value.trim().toLocaleLowerCase();
  if (!query) return;
  const menu = graph.menus.find(item => `${item.label} ${item.url}`.toLocaleLowerCase().includes(query));
  if (!menu) return;
  selectNode(graph.nodes.get(menu.url));
  graph.elements.get(menu.url)?.scrollIntoView({block:'center',inline:'center'});
});

initialise();
