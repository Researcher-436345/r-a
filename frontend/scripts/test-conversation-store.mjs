import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/shared/lib/conversation-store.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } });
const { getConversationStore, clearConversationStores } = await import(`data:text/javascript;base64,${Buffer.from(compiled.outputText).toString('base64')}`);

for (const kind of ['web', 'deep', 'paper']) {
  test(`${kind}: leaving and remounting preserves the running answer`, async () => {
    const store = getConversationStore(`${kind}:original`);
    store.setMessages([{ content: '' }]);
    store.setIsSending(true);
    store.controller = new AbortController();
    const unsubscribe = store.subscribe(() => {});
    const publishDelta = (chunk) => store.setMessages(items => [{ content: items[0].content + chunk }]);
    publishDelta('first');
    unsubscribe(); // React unmounts the original screen.
    const other = getConversationStore(`${kind}:other`);
    other.setMessages([{ content: 'different conversation' }]);
    await Promise.resolve();
    publishDelta(' second');
    const remounted = getConversationStore(`${kind}:original`);
    assert.equal(remounted.getSnapshot().isSending, true);
    assert.equal(remounted.getSnapshot().messages[0].content, 'first second');
    assert.equal(store.controller.signal.aborted, false);
    publishDelta(' final');
    store.setIsSending(false);
    assert.equal(remounted.getSnapshot().messages[0].content, 'first second final');
    assert.equal(remounted.getSnapshot().isSending, false);
    assert.equal(other.getSnapshot().messages[0].content, 'different conversation');
  });
}

test('history fetched before a new answer cannot overwrite streaming or completion', () => {
  const store = getConversationStore('race');
  const oldRevision = store.revision;
  store.setMessages([{ content: 'new answer' }]);
  store.setIsSending(true);
  store.hydrate([{ content: 'old history' }], oldRevision);
  store.setIsSending(false);
  store.hydrate([{ content: 'old history' }], oldRevision);
  assert.equal(store.getSnapshot().messages[0].content, 'new answer');
  store.hydrate([{ content: 'fresh history' }], store.revision);
  assert.equal(store.getSnapshot().messages[0].content, 'fresh history');
});

test('an old account finishing in the background cannot populate the next account', () => {
  const previous = getConversationStore('paper:same-id');
  previous.setIsSending(true);
  clearConversationStores();
  const current = getConversationStore('paper:same-id');
  previous.setMessages([{ content: 'private answer from previous account' }]);
  previous.setIsSending(false);
  assert.notEqual(previous, current);
  assert.deepEqual(current.getSnapshot().messages, []);
});
