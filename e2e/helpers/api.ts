const BASE_URL = 'http://localhost:9090';

export async function checkinViaAPI(studentName: string, deviceType = 'mobile', pin?: string) {
  const body: Record<string, string> = { student_name: studentName, device_type: deviceType };
  if (pin) body.pin = pin;
  const res = await fetch(`${BASE_URL}/api/checkin`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return res.json();
}

export async function checkoutViaAPI(studentName: string, pin?: string) {
  const body: Record<string, string> = { student_name: studentName };
  if (pin) body.pin = pin;
  const res = await fetch(`${BASE_URL}/api/checkout`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return res.json();
}

export async function getStatusViaAPI(studentName: string) {
  const res = await fetch(`${BASE_URL}/api/status?student_name=${encodeURIComponent(studentName)}`);
  return res.json();
}

export async function setPinModeViaAPI(cookie: string, mode: 'off' | 'center') {
  const res = await fetch(`${BASE_URL}/api/admin/pin/mode`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ pin_mode: mode }),
  });
  return res.json();
}

export async function setPinViaAPI(cookie: string, pin: string) {
  const res = await fetch(`${BASE_URL}/api/admin/pin`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ pin }),
  });
  return res.json();
}

export async function setStudentPinRequireViaAPI(cookie: string, studentId: string, requirePin: boolean) {
  const res = await fetch(`${BASE_URL}/api/v1/student/pin/require`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ student_id: studentId, require_pin: requirePin }),
  });
  return res.json();
}

export async function resetStudentPinViaAPI(cookie: string, studentId: string) {
  const res = await fetch(`${BASE_URL}/api/v1/student/pin/reset`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: cookie },
    body: JSON.stringify({ student_id: studentId }),
  });
  return res.json();
}

export async function getSettingsViaAPI() {
  const res = await fetch(`${BASE_URL}/api/settings`);
  return res.json();
}

export async function pinCheckViaAPI(studentId: string) {
  const res = await fetch(`${BASE_URL}/api/pin/check?student_id=${encodeURIComponent(studentId)}`);
  return res.json();
}

export async function clearStudentTrackerItemsViaAPI(cookie: string, studentId: string) {
  const res = await fetch(`${BASE_URL}/api/tracker/student-items?student_id=${encodeURIComponent(studentId)}`, {
    headers: { Cookie: cookie },
  });
  const items = await res.json();
  if (!Array.isArray(items)) return;
  for (const item of items) {
    await fetch(`${BASE_URL}/api/tracker/student-items/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ id: item.id }),
    });
  }
}

export async function adminLogin(username: string, password: string): Promise<string | null> {
  const res = await fetch(`${BASE_URL}/admin/api/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
    redirect: 'manual',
  });
  const setCookie = res.headers.get('set-cookie');
  if (!setCookie) return null;
  const match = setCookie.match(/classgo_session=([^;]+)/);
  return match ? `classgo_session=${match[1]}` : null;
}
