import cf from "cloudfront";

const KVS_ID = "${kvs_id}";
const QUEUE_JOIN_HOST = "${queue_join_host}";

async function handler(event) {
  const request = event.request;
  const kvs = cf.kvs(KVS_ID);
  let admissionSecret, sessionSecret;

  try { admissionSecret = await kvs.get("ADMISSION_SECRET"); sessionSecret = await kvs.get("SESSION_SECRET"); } catch (_) { return request; }

  if (request.cookies.q_session && await verify(request.cookies.q_session.value, sessionSecret)) return request;

  const queryAdmission = request.querystring && request.querystring.q_admission && request.querystring.q_admission.value;

  if (queryAdmission && await verify(queryAdmission, admissionSecret)) {
    request.cookies.q_admission = { value: queryAdmission };
    delete request.querystring.q_admission;
    return request;
  }

  const eventId = request.querystring && request.querystring.eventId && request.querystring.eventId.value;
  if (!eventId) return { statusCode: 400, statusDescription: "Bad Request", body: { encoding: "text", data: "eventId is required" } };

  const query = serializeQuery(request.querystring);
  const target = request.uri + (query ? "?" + query : "");

  return { statusCode: 302, statusDescription: "Found", headers: { location: { value: "https://" + QUEUE_JOIN_HOST + "/queue/join?eventId=" + encodeURIComponent(eventId) + "&target=" + encodeURIComponent(target) } } };
}

async function verify(token, secret) {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return false;
    const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["verify"]);
    return await crypto.subtle.verify("HMAC", key, decode(parts[2]), new TextEncoder().encode(parts[0] + "." + parts[1]));
  } catch (_) { return false; }
}

function decode(value) {
  const base64 = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(value.length + (4 - value.length % 4) % 4, "=");
  const binary = atob(base64);
  return Uint8Array.from(binary, c => c.charCodeAt(0));
}

function serializeQuery(querystring) {
  if (!querystring) return "";
  return Object.keys(querystring).map(key => encodeURIComponent(key) + "=" + encodeURIComponent(querystring[key].value || "")).join("&");
}
