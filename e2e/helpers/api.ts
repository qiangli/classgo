const BASE_URL = 'http://localhost:9090';

let apiCallCounter = 0;

export async function checkinViaAPI(studentName: string, deviceType = 'mobile', pin?: string) {
  // Use unique device_id per call to avoid rate limiter conflicts across tests
  const body: Record<string, string> = {
    student_name: studentName,
    device_type: deviceType,
    device_id: `e2e-${process.pid}-${++apiCallCounter}`,
  };
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

/**
 * Force checkout a student by responding to any pending signoff items first.
 * This handles the case where global tracker items block regular checkout,
 * and retries with a dummy PIN if PIN is required.
 */
export async function forceCheckoutViaAPI(studentName: string, pin?: string) {
  // First try regular checkout
  let result = await checkoutViaAPI(studentName, pin);
  if (result.ok) return result;

  // If PIN is required and none provided, retry with common test PINs
  if (result.needs_pin && !pin) {
    for (const tryPin of ['1234', '4444', '7777', '3333']) {
      result = await checkoutViaAPI(studentName, tryPin);
      if (result.ok) return result;
      if (!result.needs_pin) break; // Wrong PIN gives different error
    }
  }

  // If blocked by pending tasks, respond to them and checkout atomically
  if (result.pending_tasks && result.items) {
    const responses = result.items.map((item: any) => ({
      item_type: item.source === 'personal' ? 'personal' : 'global',
      item_id: item.id,
      item_name: item.name || '',
      status: 'done',
      notes: '',
    }));
    const body: Record<string, any> = {
      student_name: studentName,
      responses,
    };
    if (pin) body.pin = pin;
    const res = await fetch(`${BASE_URL}/api/tracker/respond`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    return res.json();
  }

  // If not checked in, get due items and try respond endpoint
  const studentId = await findStudentId(studentName);
  if (studentId) {
    const dueRes = await fetch(
      `${BASE_URL}/api/tracker/due?student_id=${encodeURIComponent(studentId)}&signoff_only=true`
    );
    const dueItems = await dueRes.json();
    if (Array.isArray(dueItems) && dueItems.length > 0) {
      const responses = dueItems.map((item: any) => ({
        item_type: item.source === 'personal' ? 'personal' : 'global',
        item_id: item.id,
        item_name: item.name || '',
        status: 'done',
        notes: '',
      }));
      const body: Record<string, any> = {
        student_name: studentName,
        responses,
      };
      if (pin) body.pin = pin;
      const res = await fetch(`${BASE_URL}/api/tracker/respond`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      return res.json();
    }
  }

  return result;
}

async function findStudentId(studentName: string): Promise<string> {
  const res = await fetch(`${BASE_URL}/api/students/search?q=${encodeURIComponent(studentName)}`);
  const students = await res.json();
  if (Array.isArray(students) && students.length > 0) return students[0].id;
  return '';
}

export async function userLogin(entityId: string, password: string): Promise<string | null> {
  // Setup (first-time) then login
  const setup = await fetch(`${BASE_URL}/api/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ entity_id: entityId, password, action: 'setup' }),
    redirect: 'manual',
  });
  const setupCookie = setup.headers.get('set-cookie');
  if (setupCookie) {
    const match = setupCookie.match(/classgo_session=([^;]+)/);
    if (match) return `classgo_session=${match[1]}`;
  }
  // Already set up — do a regular login
  const login = await fetch(`${BASE_URL}/api/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ entity_id: entityId, password, action: 'login' }),
    redirect: 'manual',
  });
  const loginCookie = login.headers.get('set-cookie');
  if (!loginCookie) return null;
  const match = loginCookie.match(/classgo_session=([^;]+)/);
  return match ? `classgo_session=${match[1]}` : null;
}

export async function reimportDataViaAPI(cookie: string) {
  await fetch(`${BASE_URL}/api/v1/import`, {
    method: 'POST',
    headers: { Cookie: cookie },
  });
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
