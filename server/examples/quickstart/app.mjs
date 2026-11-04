import { createClient } from './client.mjs';

const client = createClient();
const element = id => document.getElementById(id);
const message = text => { element('message').textContent = text; };
let busy = false;

function panels() {
  element('login-panel').hidden = Boolean(client.actor);
  element('notes-panel').hidden = !client.actor;
  element('identity').textContent = client.actor ? `Notes for ${client.actor.email}` : 'Your notes';
}
async function load() {
  const result = await client.listNotes();
  const list = element('notes');
  list.replaceChildren();
  for (const note of result.items) {
    const row = document.createElement('li');
    const title = document.createElement('strong');
    title.textContent = note.title;
    const body = document.createElement('p');
    body.textContent = note.body || '';
    row.append(title, body);
    list.append(row);
  }
  message(`Loaded ${result.items.length} note(s) for the signed-in member.`);
}
async function action(callback) {
  if (busy) return;
  busy = true;
  document.querySelectorAll('button').forEach(button => { button.disabled = true; });
  try { await callback(); } catch (error) { message(error.message); }
  finally {
    busy = false;
    document.querySelectorAll('button').forEach(button => { button.disabled = false; });
    panels();
  }
}
element('login-form').addEventListener('submit', event => {
  event.preventDefault();
  action(async () => {
    element('notes').replaceChildren();
    await client.login(element('email').value, element('password').value);
    element('password').value = '';
    panels();
    await load();
  });
});
element('note-form').addEventListener('submit', event => {
  event.preventDefault();
  action(async () => {
    await client.createNote(element('title').value, element('body').value);
    element('note-form').reset();
    await load();
  });
});
element('refresh').addEventListener('click', () => action(load));
element('logout').addEventListener('click', () => {
  if (busy) return;
  client.logout();
  element('notes').replaceChildren();
  element('note-form').reset();
  panels();
  message('Signed out. Sign in as another member to check record isolation.');
});
