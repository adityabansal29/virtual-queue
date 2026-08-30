import cf from "cloudfront";
const crypto = require("crypto");

const KVS_ID = "${kvs_id}";
const QUEUE_JOIN_HOST = "${queue_join_host}";

async function handler(event) {
  const request = event.request;
  const kvs = cf.kvs(KVS_ID);
  let admissionSecret, sessionSecret;

  try {
    admissionSecret = await kvs.get("ADMISSION_SECRET");
    sessionSecret = await kvs.get("SESSION_SECRET");
    console.log("kvsLoad=ok admissionSecretLoaded=" + !!admissionSecret + " sessionSecretLoaded=" + !!sessionSecret);
  } catch (err) {
    console.log("kvsLoad=failed error=" + (err.message || "unknown"));
    return request;
  }

  if (request.cookies.q_session && await verify(request.cookies.q_session.value, sessionSecret)) return request;

  const queryAdmission = request.querystring && request.querystring.q_admission && request.querystring.q_admission.value;
  console.log("request queryKeys=" + Object.keys(request.querystring || {}).join(",") + " admissionPresent=" + !!queryAdmission + " admissionLength=" + (queryAdmission ? queryAdmission.length : 0));

  const queryAdmissionValid = queryAdmission ? await verify(queryAdmission, admissionSecret) : false;
  console.log("queryAdmissionValid=" + queryAdmissionValid);
  if (queryAdmissionValid) {
    request.cookies.q_admission = { value: queryAdmission };
    delete request.querystring.q_admission;
    console.log("authResult=accepted");
    return request;
  }

  console.log("authResult=redirect");

  const eventId = request.querystring && request.querystring.eventId && request.querystring.eventId.value;
  if (!eventId) return { statusCode: 400, statusDescription: "Bad Request", body: { encoding: "text", data: "eventId is required" } };

  const query = serializeQuery(request.querystring);
  const target = "https://" + event.context.distributionDomainName + request.uri + (query ? "?" + query : "");

  return { statusCode: 302, statusDescription: "Found", headers: { location: { value: "https://" + QUEUE_JOIN_HOST + "/queue/join?eventId=" + encodeURIComponent(eventId) + "&target=" + encodeURIComponent(target) } } };
}

async function verify(token, secret) {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return false;
    const expected = crypto.createHmac("sha256", secret).update(parts[0] + "." + parts[1]).digest("base64url");
    return expected === parts[2];
  } catch (err) {
    console.log("verifyError=" + (err.message || "unknown"));
    return false;
  }
}

function serializeQuery(querystring) {
  if (!querystring) return "";
  return Object.keys(querystring).map(key => encodeURIComponent(key) + "=" + encodeURIComponent(querystring[key].value || "")).join("&");
}
