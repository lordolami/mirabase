// Browser/Node-compatible tutorial client for the inherited record API.
// Tokens remain in memory. This is an example, not the complete MiraBase SDK.
export function createClient(baseUrl = '') {
  const base = baseUrl.replace(/\/+$/, '');
  let token = '';
  let actor = null;

  async function request(path, options = {}) {
    const headers = new Headers(options.headers);
    headers.set('Accept', 'application/json');
    if (options.body) headers.set('Content-Type', 'application/json');
    if (token) headers.set('Authorization', token);
    const response = await fetch(base + path, { ...options, headers });
    const body = await response.json();
    if (!response.ok) {
      const error = new Error(body.message || `Request failed (${response.status})`);
      error.status = response.status;
      error.response = body;
      throw error;
    }
    return body;
  }

  return {
    get actor() { return actor; },
    async login(identity, password) {
      token = '';
      actor = null;
      const result = await request('/api/collections/members/auth-with-password', {
        method: 'POST', body: JSON.stringify({ identity, password }),
      });
      token = result.token;
      actor = result.record;
      return actor;
    },
    logout() { token = ''; actor = null; },
    async listNotes() {
      if (!actor) throw new Error('Sign in first.');
      return request('/api/collections/notes/records?page=1&perPage=20&sort=-created');
    },
    async createNote(title, body) {
      if (!actor) throw new Error('Sign in first.');
      return request('/api/collections/notes/records', {
        method: 'POST', body: JSON.stringify({ title, body, owner: actor.id }),
      });
    },
  };
}
