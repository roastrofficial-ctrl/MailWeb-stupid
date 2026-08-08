"use strict";

const elements = {
    address: document.getElementById("address"),
    addressForm: document.getElementById("address-form"),
    back: document.getElementById("back"),
    forward: document.getElementById("forward"),
    reload: document.getElementById("reload"),
    loading: document.getElementById("loading"),
    error: document.getElementById("error"),
    viewport: document.getElementById("viewport"),
};

elements.addressForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void navigate({uri: elements.address.value});
});
elements.back.addEventListener("click", () => void action("/api/back"));
elements.forward.addEventListener("click", () => void action("/api/forward"));
elements.reload.addEventListener("click", () => void action("/api/reload"));

async function navigate(payload) {
    await requestState("/api/navigate", payload);
}

async function action(endpoint) {
    await requestState(endpoint, {});
}

async function requestState(endpoint, payload) {
    setLoading(true);
    showError("");
    try {
        const response = await fetch(endpoint, {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify(payload),
        });
        const result = await response.json();
        if (result.state) updateState(result.state);
        if (!response.ok) throw new Error(result.error || "Navigation failed");
    } catch (error) {
        showError(error instanceof Error ? error.message : String(error));
    } finally {
        setLoading(false);
    }
}

function updateState(state) {
    setText("client-mailbox", state.client_mailbox || "No mailbox — direct transport");
    elements.back.disabled = !state.can_go_back;
    elements.forward.disabled = !state.can_go_forward;
    elements.reload.disabled = !state.current;
    if (!state.current) return;
    elements.address.value = state.current.uri;
    renderDocument(state.current.response.document, state.current.uri);
    renderDebug(state.current);
}

function renderDocument(mailwebDocument, currentURI) {
    elements.viewport.replaceChildren();
    document.title = `${mailwebDocument.title} — Postbox`;
    const page = document.createElement("div");
    page.className = "mailweb-document";
    for (const node of mailwebDocument.body) {
        page.appendChild(renderNode(node, currentURI));
    }
    elements.viewport.appendChild(page);
}

function renderNode(node, currentURI) {
    switch (node.type) {
    case "heading": {
        const heading = document.createElement(`h${node.level}`);
        heading.textContent = node.text;
        return heading;
    }
    case "paragraph": {
        const paragraph = document.createElement("p");
        paragraph.textContent = node.text;
        return paragraph;
    }
    case "link":
    case "button": {
        const wrapper = document.createElement("p");
        const control = document.createElement("button");
        control.type = "button";
        control.className = node.type === "button" ? "document-button" : "document-link";
        control.textContent = node.label;
        if (isMailWebReference(node.href, currentURI)) {
            control.addEventListener("click", () => void navigate({reference: node.href}));
        } else {
            control.disabled = true;
            control.title = "External navigation is disabled by Postbox";
        }
        wrapper.appendChild(control);
        return wrapper;
    }
    case "image": {
        const figure = document.createElement("figure");
        const resolved = safeURL(node.src, currentURI);
        if (resolved && (resolved.protocol === "http:" || resolved.protocol === "https:")) {
            const image = document.createElement("img");
            image.src = resolved.href;
            image.alt = node.alt;
            image.loading = "lazy";
            figure.appendChild(image);
        } else {
            const placeholder = document.createElement("div");
            placeholder.className = "image-placeholder";
            placeholder.textContent = node.alt || "MailWeb image";
            figure.appendChild(placeholder);
        }
        return figure;
    }
    default: {
        const unsupported = document.createElement("p");
        unsupported.textContent = "Unsupported document node.";
        return unsupported;
    }
    }
}

function isMailWebReference(reference, currentURI) {
    const resolved = safeURL(reference, currentURI);
    return Boolean(resolved && resolved.protocol === "mailweb:");
}

function safeURL(reference, base) {
    try {
        return new URL(reference, base);
    } catch {
        return null;
    }
}

function renderDebug(current) {
    setText("debug-uri", current.uri);
    setText("debug-request-id", current.request.id);
    setText("debug-status", String(current.response.status));
    setText("debug-transport", current.transport);
    setText("debug-sent", formatTime(current.request_sent_at));
    setText("debug-received", formatTime(current.response_received_at));
    setText("debug-duration", `${current.round_trip_ms} ms`);
    setText("debug-request", JSON.stringify(current.request, null, 2));
    setText("debug-response", JSON.stringify(current.response, null, 2));
}

function setText(id, value) {
    document.getElementById(id).textContent = value;
}

function formatTime(value) {
    return new Date(value).toLocaleTimeString([], {hour: "2-digit", minute: "2-digit", second: "2-digit", fractionalSecondDigits: 3});
}

function setLoading(active) {
    elements.loading.hidden = !active;
    elements.addressForm.querySelector("button").disabled = active;
    elements.address.disabled = active;
}

function showError(message) {
    elements.error.textContent = message;
    elements.error.hidden = !message;
}

fetch("/api/state", {headers: {"Accept": "application/json"}})
    .then((response) => response.json())
    .then((result) => updateState(result.state))
    .catch((error) => showError(String(error)));
