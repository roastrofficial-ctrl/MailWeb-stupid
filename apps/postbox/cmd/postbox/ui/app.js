"use strict";

const elements = Object.fromEntries(["address", "address-form", "back", "forward", "reload", "loading", "error", "viewport", "correspondence-stage", "travelling-envelope", "correspondence-status", "route-host", "route-mailbox", "postbox-open", "postbox-drawer", "postbox-items", "postbox-count", "stationery-items", "correspondence-view-open", "correspondence-view", "email-request", "email-response", "journey-view-open", "journey-view", "journey-history", "journey-summary", "journey-events", "journey-instant"].map((id) => [id, document.getElementById(id)]));
let currentState = null;
let appearanceMode = "correspondent";
let knownArchive = new Set();

class CorrespondenceAnimation {
    constructor(stage) { this.stage = stage; this.timer = null; this.reduced = matchMedia("(prefers-reduced-motion: reduce)").matches; }
    start(kind, uri) {
        clearTimeout(this.timer);
        const parsed = safeURL(uri, currentState?.current?.uri || "mailweb://unknown.local/");
        elements["route-host"].textContent = parsed?.host || "correspondent";
        elements["route-mailbox"].textContent = currentState?.client_mailbox || "private mailbox";
        this.stage.hidden = false;
		elements["postbox-open"].classList.toggle("is-retrieving", kind === "archived");
        this.stage.dataset.phase = kind === "archived" ? "retrieve" : "send";
        elements["correspondence-status"].textContent = kind === "archived" ? "Already in your postbox — retrieving correspondence…" : `Writing privately to ${parsed?.host || "your correspondent"}…`;
        if (kind === "live") this.timer = setTimeout(() => { this.stage.dataset.phase = "waiting"; elements["correspondence-status"].textContent = "Awaiting private correspondence…"; }, 520);
    }
    async finish(current) {
        clearTimeout(this.timer);
		const archived = current.delivery !== "live";
        this.stage.dataset.phase = archived ? "open-archived" : "receive";
        elements["correspondence-status"].textContent = archived
            ? `Correspondence already received · filed earlier · opened in ${current.navigation_ms}ms`
			: postalOutcome(current);
        await delay(this.reduced ? 80 : archived ? 520 : 760);
        this.stage.dataset.phase = "open";
        await delay(this.reduced ? 50 : 260);
        this.stage.hidden = true;
		elements["postbox-open"].classList.remove("is-retrieving");
    }
	fail(message) { clearTimeout(this.timer); elements["postbox-open"].classList.remove("is-retrieving"); this.stage.dataset.phase = "error"; elements["correspondence-status"].textContent = message; setTimeout(() => { this.stage.hidden = true; }, 1400); }
}

class PresentationResolver {
    apply(mailwebDocument, mode) {
        const viewport = elements.viewport;
        viewport.className = `viewport appearance-${mode}`;
        for (const name of ["accent", "background", "foreground", "surface", "radius", "spacing", "font"]) viewport.style.removeProperty(`--document-${name}`);
        if (mode !== "correspondent") return;
        const hint = mailwebDocument.presentation || {};
        for (const name of ["accent", "background", "foreground", "surface"]) if (/^#[0-9a-f]{6}$/i.test(hint[name] || "")) viewport.style.setProperty(`--document-${name}`, hint[name]);
        const fonts = {system: "ui-sans-serif, system-ui, sans-serif", editorial: "Georgia, 'Times New Roman', serif", sans: "Arial, ui-sans-serif, sans-serif", mono: "'SFMono-Regular', Consolas, monospace"};
        const spacing = {compact: ".76", comfortable: "1", spacious: "1.22"};
        const radius = {square: "0px", soft: "10px", round: "28px"};
        if (fonts[hint.typeface]) viewport.style.setProperty("--document-font", fonts[hint.typeface]);
        if (spacing[hint.density]) viewport.style.setProperty("--document-spacing", spacing[hint.density]);
        if (radius[hint.corners]) viewport.style.setProperty("--document-radius", radius[hint.corners]);
    }
}

class PostboxDrawer {
    render(archive, stationery = []) {
        elements["postbox-items"].replaceChildren();
        elements["postbox-count"].textContent = String(archive.length);
        for (const item of archive) {
            const article = document.createElement("article"); article.className = "postbox-item"; article.dataset.uri = item.uri;
            if (item.current) article.classList.add("is-current");
            const title = document.createElement("h3"); title.textContent = item.title;
            const uri = document.createElement("code"); uri.textContent = item.uri;
            const meta = document.createElement("p"); meta.textContent = item.current ? "Currently reading" : `${item.delivery} · received ${relativeTime(item.received_at)} · original trip ${item.round_trip_ms}ms`;
            article.append(title, uri, meta);
            if (!item.current) { const open = document.createElement("button"); open.type = "button"; open.textContent = "Retrieve letter"; open.addEventListener("click", () => { elements["postbox-drawer"].close(); void navigate({uri: item.uri}); }); article.appendChild(open); }
            elements["postbox-items"].appendChild(article);
        }
		elements["stationery-items"].replaceChildren();
		for (const file of stationery) { const article = document.createElement("article"), mark = document.createElement("span"), title = document.createElement("h3"), id = document.createElement("code"), meta = document.createElement("p"), details = rawDetails("View semantic stationery", file.document); article.className = "stationery-item"; mark.className = "stationery-mark"; mark.textContent = "▧"; title.textContent = file.title; id.textContent = file.id; meta.textContent = `received ${relativeTime(file.received_at)} · ${file.version.slice(0, 18)}… · used by ${file.uses?.length || 0} page(s)`; article.append(mark, title, id, meta, details); elements["stationery-items"].appendChild(article); }
    }
}

class CorrespondenceView {
    render(current) {
        elements["email-request"].replaceChildren(this.emailCard("OUTGOING", requestEmail(current)));
        elements["email-response"].replaceChildren(this.emailCard("REPLY", responseEmail(current)));
    }
    emailCard(folder, message) {
        const card = document.createElement("article"); card.className = "email-card";
        const folderLabel = document.createElement("p"); folderLabel.className = "email-folder"; folderLabel.textContent = folder;
        const subject = document.createElement("h3"); subject.textContent = message.subject;
        const metadata = document.createElement("dl");
        for (const [label, value] of [["From", message.from], ["To", message.to], [message.timeLabel, message.time]]) { const row = document.createElement("div"), dt = document.createElement("dt"), dd = document.createElement("dd"); dt.textContent = label; dd.textContent = value; row.append(dt, dd); metadata.appendChild(row); }
        const body = document.createElement("div"); body.className = "email-body";
        for (const line of message.lines) { const p = document.createElement("p"); p.textContent = line; body.appendChild(p); }
        const attachment = document.createElement("details"); const summary = document.createElement("summary"); summary.textContent = message.attachmentLabel; const raw = document.createElement("pre"); raw.textContent = JSON.stringify(message.raw, null, 2); attachment.append(summary, raw);
        card.append(folderLabel, subject, metadata, body, attachment); return card;
    }
}

const animation = new CorrespondenceAnimation(elements["correspondence-stage"]);
const presentation = new PresentationResolver();
const postbox = new PostboxDrawer();
class JourneyInspector {
    constructor() { this.timers = []; this.selected = null; }
    render(state, selectedID, instant = false) {
        this.timers.forEach(clearTimeout); this.timers = [];
        const journeys = state.journeys || [], selected = journeys.find((item) => item.id === selectedID) || state.last_journey;
        if (!selected) return; this.selected = selected.id;
        elements["journey-history"].replaceChildren();
        for (const journey of [...journeys].reverse()) { const button = document.createElement("button"); button.type = "button"; button.className = journey.id === selected.id ? "selected" : ""; button.textContent = `${journey.method} ${safeURL(journey.uri)?.pathname || journey.uri} · ${journey.outcome}`; button.addEventListener("click", () => this.render(state, journey.id)); elements["journey-history"].appendChild(button); }
        elements["journey-summary"].replaceChildren(...[`${selected.method} ${selected.uri}`, `${selected.transport} · ${selected.delivery}`, `${selected.outcome.toUpperCase()} · ${selected.round_trip_ms}ms`, selected.id].map((line) => { const p = document.createElement("p"); p.textContent = line; return p; }));
        const original = selected.original_journey_id && journeys.find((item) => item.id === selected.original_journey_id);
        const stream = original ? [...original.events.map((event) => ({event, owner: original})), {event: {type: "archive.interval", label: "… later: correspondence retrieved from Postbox", timestamp: selected.started_at}, owner: selected}, ...selected.events.map((event) => ({event, owner: selected}))] : selected.events.map((event) => ({event, owner: selected}));
        elements["journey-events"].replaceChildren(); const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
        stream.forEach(({event, owner}, index) => { const item = this.event(event, owner); item.hidden = !(instant || reduced); elements["journey-events"].appendChild(item); if (item.hidden) this.timers.push(setTimeout(() => { item.hidden = false; }, 90 * index)); });
    }
    event(event, journey) {
        const item = document.createElement("li"), code = document.createElement("code"), body = document.createElement("div"), label = document.createElement("strong"), time = document.createElement("time");
        const eventCodes = {navigation: "NAV", cache: "BOX", premail: "PRE", request: "REQ", transport: "SMTP", mailbox: "MAIL", response: "MW", template: "TPL", stationery: "BOX", document: "DOC", form: "FORM"}; code.textContent = eventCodes[event.type.split(".")[0]] || "EVENT"; label.textContent = event.label; time.textContent = formatTime(event.timestamp); body.append(label, time);
        if (event.metadata) { const meta = document.createElement("small"); meta.textContent = Object.entries(event.metadata).map(([key, value]) => `${key}=${value}`).join(" · "); body.appendChild(meta); }
        if (event.type === "request.created" && journey.request) body.appendChild(rawDetails("Actual MailWebRequest", journey.request));
        if (event.type === "response.received" && journey.response) body.appendChild(rawDetails("Actual MailWebResponse", journey.response));
        item.append(code, body); return item;
    }
}

const correspondence = new CorrespondenceView();
const journeyInspector = new JourneyInspector();

elements["address-form"].addEventListener("submit", (event) => { event.preventDefault(); void navigate({uri: elements.address.value}); });
elements.back.addEventListener("click", () => void action("/api/back", "live"));
elements.forward.addEventListener("click", () => void action("/api/forward", "live"));
elements.reload.addEventListener("click", () => void action("/api/reload", "live"));
elements["postbox-open"].addEventListener("click", () => elements["postbox-drawer"].showModal());
elements["journey-view-open"].addEventListener("click", () => { if (currentState?.last_journey) { journeyInspector.render(currentState); elements["journey-view"].showModal(); } });
elements["journey-instant"].addEventListener("click", () => journeyInspector.render(currentState, journeyInspector.selected, true));
elements["correspondence-view-open"].addEventListener("click", () => { if (currentState?.current) { correspondence.render(currentState.current); elements["correspondence-view"].showModal(); } });
for (const button of document.querySelectorAll("[data-close]")) button.addEventListener("click", () => document.getElementById(button.dataset.close).close());
for (const input of document.querySelectorAll('input[name="appearance"]')) input.addEventListener("change", () => { appearanceMode = input.value; if (currentState?.current) { presentation.apply(currentState.current.response.document || {}, appearanceMode); renderDocument(currentState.current); } });

async function navigate(payload) {
    let target = payload.uri || resolveReference(payload.reference);
    await requestState("/api/navigate", payload, target && knownArchive.has(target) ? "archived" : "live", target);
}
async function action(endpoint, kind) { await requestState(endpoint, {}, kind, currentState?.current?.uri); }
async function submitForm(node, values) { await requestState("/api/form", {method: node.method, action: node.action, values}, "live", resolveReference(node.action)); }

async function requestState(endpoint, payload, kind, target) {
    animation.start(kind, target || "mailweb://correspondent/"); setLoading(true); showError("");
    try {
        const response = await fetch(endpoint, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)});
        const result = await response.json();
        if (!response.ok) { if (result.state) { currentState = result.state; elements["journey-view-open"].disabled = !result.state.last_journey; if (result.state.retained_correspondence) setText("debug-response", JSON.stringify(result.state.retained_correspondence, null, 2)); } throw new Error(result.error || "Navigation failed"); }
        await animation.finish(result.state.current);
        updateState(result.state);
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error); animation.fail(message.includes("timed out") ? "No private reply received." : "Correspondence could not be delivered."); renderTransportFailure(target, message); showError(message);
    } finally { setLoading(false); }
}

function updateState(state, backgroundOnly = false) {
    const newArchive = new Set((state.archive || []).map((item) => item.uri));
    const arrivals = [...newArchive].filter((uri) => !knownArchive.has(uri) && uri !== state.current?.uri);
    if (arrivals.length) { elements["postbox-open"].classList.add("has-arrival"); setTimeout(() => elements["postbox-open"].classList.remove("has-arrival"), 1000); }
    knownArchive = newArchive; currentState = state; postbox.render(state.archive || [], state.stationery || []);
	renderPremailLetters(state.premail?.targets || []);
    setText("client-mailbox", state.client_mailbox || "No mailbox — direct transport");
    setText("premail-status", state.notice || state.premail?.message || "prEmail: idle.");
    if (backgroundOnly) return;
    elements.back.disabled = !state.can_go_back; elements.forward.disabled = !state.can_go_forward; elements.reload.disabled = !state.current;
    if (!state.current) return;
    elements.address.value = state.current.uri; presentation.apply(state.current.response.document || {}, appearanceMode); renderDocument(state.current); renderDebug(state.current);
    elements["correspondence-view-open"].disabled = false;
	elements["journey-view-open"].disabled = !state.last_journey;
}

function renderPremailLetters(targets) {
	const area = document.getElementById("premail-letters"); area.replaceChildren();
	for (const target of targets) { const chip = document.createElement("span"), uri = safeURL(target); chip.textContent = `letter → ${uri?.pathname || target}`; area.appendChild(chip); }
}

function renderDocument(current) {
    const mailwebDocument = current.response.document || {title: "No enclosure", body: []}, currentURI = current.uri, delivery = current.delivery;
    elements.viewport.replaceChildren(); document.title = `${mailwebDocument.title} — Postbox`;
    const page = document.createElement("div"); page.className = "mailweb-document";
	const arrival = document.createElement("p"); arrival.className = "arrival-note"; arrival.textContent = delivery !== "live" ? "PRIVATE EMAIL ALREADY RECEIVED — RETRIEVED FROM YOUR POSTBOX" : "PRIVATE EMAIL RECEIVED — UNSEALED AND RENDERED BY POSTBOX"; page.appendChild(arrival);
    if (current.stationery_status) { const stationery = document.createElement("p"); stationery.className = "stationery-note"; stationery.textContent = current.stationery_status; page.appendChild(stationery); }
    const postal = postalState(current); if (postal) page.appendChild(renderPostalState(postal, current));
    for (const node of mailwebDocument.body) page.appendChild(renderNode(node, currentURI));
    elements.viewport.appendChild(page);
}

function renderNode(node, currentURI) {
    switch (node.type) {
    case "heading": { const heading = document.createElement(`h${node.level}`); heading.textContent = node.text; if (node.variant === "display") heading.className = "display-heading"; return heading; }
    case "paragraph": { const paragraph = document.createElement("p"); paragraph.textContent = node.text; return paragraph; }
    case "link": case "button": { const wrapper = document.createElement("p"), control = document.createElement("button"); control.type = "button"; control.className = node.type === "button" ? "document-button" : "document-link"; if (node.variant === "prominent") control.classList.add("prominent"); control.textContent = node.label; if (isMailWebReference(node.href, currentURI)) control.addEventListener("click", () => void navigate({reference: node.href})); else { control.disabled = true; control.title = "External navigation is disabled by Postbox"; } wrapper.appendChild(control); return wrapper; }
    case "image": { const figure = document.createElement("figure"); if (node.variant === "hero") figure.className = "hero-image"; const resolved = safeURL(node.src, currentURI); if (resolved && (resolved.protocol === "http:" || resolved.protocol === "https:")) { const image = document.createElement("img"); image.src = resolved.href; image.alt = node.alt; image.loading = "lazy"; figure.appendChild(image); } else { const placeholder = document.createElement("div"); placeholder.className = "image-placeholder"; placeholder.textContent = node.alt || "MailWeb image"; figure.appendChild(placeholder); } return figure; }
    case "nav": { const nav = document.createElement("nav"), label = document.createElement("span"), list = document.createElement("ul"); nav.className = "mailweb-nav"; nav.setAttribute("aria-label", node.label); label.className = "visually-hidden"; label.textContent = node.label; for (const item of node.items) { const li = document.createElement("li"), control = document.createElement("button"), destination = resolveReference(item.href); control.type = "button"; control.textContent = item.label; if (destination === currentURI) { control.className = "current"; control.setAttribute("aria-current", "page"); } control.addEventListener("click", () => void navigate({reference: item.href})); li.appendChild(control); list.appendChild(li); } nav.append(label, list); return nav; }
    case "form": { const form = document.createElement("form"); form.className = "mailweb-form"; for (const field of node.fields) { const group = document.createElement("label"); group.className = "form-field"; const label = document.createElement("span"); label.textContent = field.label; const input = document.createElement("input"); input.type = "text"; input.name = field.name; input.placeholder = field.placeholder || ""; input.required = Boolean(field.required); input.autocomplete = "off"; group.append(label, input); form.appendChild(group); } const submit = document.createElement("button"); submit.type = "submit"; submit.className = "document-button"; submit.textContent = node.submit; form.appendChild(submit); form.addEventListener("submit", (event) => { event.preventDefault(); const values = Object.create(null), data = new FormData(form); for (const field of node.fields) values[field.name] = String(data.get(field.name) || ""); void submitForm(node, values); }); return form; }
    default: { const unsupported = document.createElement("p"); unsupported.textContent = "Unsupported document node."; return unsupported; }
    }
}

function requestEmail(current) {
    const uri = new URL(current.request.uri), body = current.request.body || {};
    const lines = [`Dear ${uri.host},`, current.request.method === "POST" ? "Please find enclosed the following private information:" : "Would you kindly send me:", current.request.method === "POST" ? Object.entries(body).map(([key, value]) => `${key}: ${String(value)}`).join(" · ") : uri.pathname + uri.search, `Method: ${current.request.method} · Protocol: MailWeb ${current.request.mailweb}`, "I await your private correspondence.", "Yours faithfully, Postbox"];
    return {subject: `MailWeb correspondence: ${uri.pathname}`, from: current.client_mailbox || "Postbox", to: current.publisher_mailbox || uri.host, timeLabel: "Sent", time: formatTime(current.request_sent_at), lines, attachmentLabel: current.request.method === "POST" ? "Enclosed data · form-data.mailweb.json" : "Actual MailWebRequest JSON", raw: current.request};
}
function responseEmail(current) {
    const uri = new URL(current.uri), document = current.response.document || {title: "No enclosure", body: []}, counts = {}; for (const node of document.body) counts[node.type] = (counts[node.type] || 0) + 1;
    const inventory = Object.entries(counts).map(([type, count]) => `${count} ${type}${count === 1 ? "" : "s"}`).join(" · ");
    const raw = current.raw_response || current.response, template = raw.document?.template, newlyEnclosed = Boolean(template && raw.templates?.some((item) => item.id === template && item.version === raw.document.template_version));
    const stationeryLine = template ? (newlyEnclosed ? `I have also enclosed our current stationery (${template}) for future correspondence.` : `Please use stationery already on file: ${template} · ${raw.document.template_version}`) : "No stationery reference accompanied this reply.";
    return {subject: `Re: MailWeb correspondence: ${uri.pathname}`, from: current.publisher_mailbox || uri.host, to: current.client_mailbox || "Postbox", timeLabel: "Received", time: formatTime(current.response_received_at), lines: ["Dear Postbox,", current.response.document ? "Certainly. Please find the requested semantic content enclosed:" : "Your message was received without a document enclosure.", stationeryLine, `“${document.title}”`, `Status: ${current.response.status} · Protocol: MailWeb ${current.response.mailweb}`, inventory, "Yours,", uri.host], attachmentLabel: newlyEnclosed ? "Content and stationery · actual MailWebResponse JSON" : "Content enclosure · actual MailWebResponse JSON", raw};
}


function postalState(current) {
    const status = current.response.status;
    const states = {201: ["RECEIVED", "Correspondence acted upon", "Your correspondence has been received and acted upon."], 202: ["ACCEPTED", "Correspondence accepted", "The request is being processed; this does not mean it is complete."], 204: ["NO ENCLOSURE", "No reply enclosed", "Your correspondent received the message, but enclosed no document."], 400: ["UNREADABLE", "Your letter could not be understood", "The correspondent could not interpret this request."], 401: ["IDENTIFICATION REQUIRED", "Correspondence withheld", "Identification is required before this correspondence can be released."], 403: ["DECLINED", "Correspondence declined", "Your correspondent declined to provide this document."], 404: ["RETURN TO SENDER", "Address unknown", `The requested correspondence could not be found at ${current.uri}`], 409: ["CONFLICT", "Already on file", "Your correspondence conflicts with something already on file."], 429: ["TOO MUCH CORRESPONDENCE", "Please try again shortly", "Your correspondent has asked for a short break before receiving more post."]};
    if (states[status]) return states[status];
    if (status >= 500) return ["WE REGRET TO INFORM YOU", status === 502 || status === 503 ? "Correspondent temporarily unavailable" : "Reply could not be prepared", "Your correspondent was unable to prepare a reply."];
    if (status >= 300 && status < 400) return ["FORWARDING", "Forwarding address received", "The technical response is preserved below; automatic forwarding is not invented."];
    return null;
}
function renderPostalState(postal, current) { const card = document.createElement("section"), stamp = document.createElement("strong"), title = document.createElement("h1"), copy = document.createElement("p"), technical = document.createElement("code"); card.className = `postal-state status-${current.response.status}`; stamp.textContent = postal[0]; title.textContent = postal[1]; copy.textContent = postal[2]; technical.textContent = `MAILWEB STATUS ${current.response.status}`; card.append(stamp, title, copy, technical); return card; }
function renderTransportFailure(uri, message) { const timeout = message.toLowerCase().includes("timed out"), missing = message.toLowerCase().includes("missing stationery"), card = document.createElement("section"), stamp = document.createElement("strong"), title = document.createElement("h1"), copy = document.createElement("p"), retry = document.createElement("button"); card.className = "postal-state transport-failure"; stamp.textContent = missing ? "MISSING STATIONERY" : timeout ? "NO REPLY" : "DAMAGED"; title.textContent = missing ? "The right paper is not in your Postbox" : timeout ? "No correspondence received" : "This correspondence arrived damaged"; copy.textContent = missing ? `${message}. The correspondence was not composed using a different version.` : timeout ? `We wrote to ${safeURL(uri)?.host || "the correspondent"}, but no correlated reply arrived before the real transport timeout.` : "Postbox could not transport, decode, or validate the correspondence. The technical error is shown below."; retry.type = "button"; retry.className = "document-button"; retry.textContent = "Send again"; retry.addEventListener("click", () => void navigate({uri})); card.append(stamp, title, copy, retry); elements.viewport.replaceChildren(card); }
function rawDetails(label, value) { const details = document.createElement("details"), summary = document.createElement("summary"), pre = document.createElement("pre"); summary.textContent = label; pre.textContent = JSON.stringify(value, null, 2); details.append(summary, pre); return details; }

function postalOutcome(current) {
	const status = current.response.status;
	if (status === 404) return "Return to sender — address unknown · status 404";
	if (status === 429) return "Too much correspondence — please try again shortly · status 429";
	if (status >= 500) return `Your correspondent was unable to prepare a reply · status ${status}`;
	if (status >= 300 && status < 400) return `Forwarding address received · status ${status}`;
	return `Private correspondence received · ${current.round_trip_ms}ms`;
}

function renderDebug(current) { setText("debug-uri", current.uri); setText("debug-request-id", current.request.id); setText("debug-status", String(current.response.status)); setText("debug-transport", current.transport); setText("debug-method", current.request.method); setText("debug-delivery", current.delivery); setText("debug-sent", formatTime(current.request_sent_at)); setText("debug-received", formatTime(current.response_received_at)); setText("debug-duration", `${current.round_trip_ms} ms`); setText("debug-navigation", `${current.navigation_ms} ms`); setText("debug-prefetched", current.prefetched_at ? formatTime(current.prefetched_at) : "—"); setText("debug-opened", formatTime(current.opened_at)); setText("debug-request", JSON.stringify(current.request, null, 2)); setText("debug-response", JSON.stringify(current.raw_response || current.response, null, 2)); }
function resolveReference(reference) { try { return new URL(reference, currentState?.current?.uri).href; } catch { return reference; } }
function isMailWebReference(reference, currentURI) { const resolved = safeURL(reference, currentURI); return Boolean(resolved && resolved.protocol === "mailweb:"); }
function safeURL(reference, base) { try { return new URL(reference, base); } catch { return null; } }
function relativeTime(value) { const seconds = Math.max(0, Math.round((Date.now() - new Date(value).getTime()) / 1000)); return seconds < 2 ? "just now" : `${seconds}s ago`; }
function setText(id, value) { document.getElementById(id).textContent = value; }
function formatTime(value) { return new Date(value).toLocaleTimeString([], {hour: "2-digit", minute: "2-digit", second: "2-digit", fractionalSecondDigits: 3}); }
function setLoading(active) { elements.loading.hidden = !active; elements["address-form"].querySelector("button").disabled = active; elements.address.disabled = active; }
function showError(message) { elements.error.textContent = message; elements.error.hidden = !message; }
function delay(ms) { return new Promise((resolve) => setTimeout(resolve, ms)); }

fetch("/api/state", {headers: {"Accept": "application/json"}}).then((response) => response.json()).then((result) => updateState(result.state)).catch((error) => showError(String(error)));
setInterval(() => fetch("/api/state", {headers: {"Accept": "application/json"}}).then((response) => response.json()).then((result) => updateState(result.state, true)).catch(() => {}), 500);
