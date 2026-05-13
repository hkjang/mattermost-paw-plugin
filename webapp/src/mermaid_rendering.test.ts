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
        'Intro sentence',
        '```mermaid',
        'graph TD',
        'A-->B',
        '```',
        'Closing sentence',
    ].join('\n'));

    expect(segments).toEqual([
        {kind: 'text', content: 'Intro sentence\n'},
        {kind: 'mermaid', content: 'graph TD\nA-->B'},
        {kind: 'text', content: '\nClosing sentence'},
    ]);
});

test('splitRenderableMessage keeps plain text when mermaid fence is incomplete', () => {
    const message = '```mermaid\ngraph TD\nA-->B';
    expect(splitRenderableMessage(message)).toEqual([{kind: 'text', content: message}]);
});

test('normalizeRenderableMessage removes blank lines inside markdown tables', () => {
    const normalized = normalizeRenderableMessage([
        'Before table.',
        '',
        '    | Name | Value |',
        '',
        '    | --- | --- |',
        '',
        '    | A | 1 |',
        '',
        'After table.',
    ].join('\n'));

    expect(normalized).toBe([
        'Before table.',
        '',
        '| Name | Value |',
        '| --- | --- |',
        '| A | 1 |',
        '',
        'After table.',
    ].join('\n'));
});

test('splitRenderableMessage detects indented mermaid fences after normalization', () => {
    const segments = splitRenderableMessage([
        'Description',
        '    ```mermaid',
        '    graph TD',
        '    A-->B',
        '    ```',
    ].join('\n'));

    expect(segments).toEqual([
        {kind: 'text', content: 'Description\n'},
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
        '| Name | Value |',
        '',
        '| --- | --- |',
        '```',
    ].join('\n'));

    expect(normalized).toBe([
        '```text',
        '| Name | Value |',
        '',
        '| --- | --- |',
        '```',
    ].join('\n'));
});
