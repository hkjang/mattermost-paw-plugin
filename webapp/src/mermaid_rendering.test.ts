jest.mock('mermaid/dist/mermaid.esm.min.mjs', () => ({
    __esModule: true,
    default: {
        initialize: jest.fn(),
        render: jest.fn().mockResolvedValue({svg: '<svg></svg>'}),
    },
}));

import {containsCompleteMermaidFence, normalizeRenderableMessage, splitRenderableMessage} from './mermaid_rendering';

test('containsCompleteMermaidFence matches only closed mermaid fences', () => {
    expect(containsCompleteMermaidFence('```mermaid\ngraph TD\nA-->B\n```')).toBe(true);
    expect(containsCompleteMermaidFence('```mermaid\ngraph TD\nA-->B')).toBe(false);
});

test('splitRenderableMessage separates text and mermaid segments', () => {
    const segments = splitRenderableMessage([
        '?쒕몢 臾몄옣',
        '```mermaid',
        'graph TD',
        'A-->B',
        '```',
        '留덈Т由?臾몄옣',
    ].join('\n'));

    expect(segments).toEqual([
        {kind: 'text', content: '?쒕몢 臾몄옣\n'},
        {kind: 'mermaid', content: 'graph TD\nA-->B'},
        {kind: 'text', content: '\n留덈Т由?臾몄옣'},
    ]);
});

test('splitRenderableMessage keeps plain text when mermaid fence is incomplete', () => {
    const message = '```mermaid\ngraph TD\nA-->B';
    expect(splitRenderableMessage(message)).toEqual([{kind: 'text', content: message}]);
});

test('normalizeRenderableMessage removes blank lines inside markdown tables', () => {
    const normalized = normalizeRenderableMessage([
        '?쒖엯?덈떎.',
        '',
        '    | ?대쫫 | 媛?|',
        '',
        '    | --- | --- |',
        '',
        '    | A | 1 |',
        '',
        '?ㅼ쓬 臾몄옣',
    ].join('\n'));

    expect(normalized).toBe([
        '?쒖엯?덈떎.',
        '',
        '| ?대쫫 | 媛?|',
        '| --- | --- |',
        '| A | 1 |',
        '',
        '?ㅼ쓬 臾몄옣',
    ].join('\n'));
});

test('splitRenderableMessage detects indented mermaid fences after normalization', () => {
    const segments = splitRenderableMessage([
        '?ㅻ챸',
        '    ```mermaid',
        '    graph TD',
        '    A-->B',
        '    ```',
    ].join('\n'));

    expect(segments).toEqual([
        {kind: 'text', content: '?ㅻ챸\n'},
        {kind: 'mermaid', content: 'graph TD\nA-->B'},
    ]);
});

test('normalizeRenderableMessage removes one extra blank line inside fenced code blocks', () => {
    const normalized = normalizeRenderableMessage([
        '```python',
        '',
        'print("hello")',
        '',
        '```',
    ].join('\n'));

    expect(normalized).toBe([
        '```python',
        'print("hello")',
        '```',
    ].join('\n'));
});

test('normalizeRenderableMessage does not rewrite markdown table syntax inside fenced code blocks', () => {
    const normalized = normalizeRenderableMessage([
        '```text',
        '| ?대쫫 | 媛?|',
        '',
        '| --- | --- |',
        '```',
    ].join('\n'));

    expect(normalized).toBe([
        '```text',
        '| ?대쫫 | 媛?|',
        '',
        '| --- | --- |',
        '```',
    ].join('\n'));
});
