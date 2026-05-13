import React, {useEffect, useRef, useState} from 'react';

import {renderMermaidDefinition} from '../mermaid_rendering';

type Props = {
    definition: string;
    postID: string;
    index: number;
};

export default function MermaidDiagram({definition, postID, index}: Props) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const popupContainerRef = useRef<HTMLDivElement | null>(null);
    const copyResetTimerRef = useRef<number | null>(null);
    const [error, setError] = useState('');
    const [popupError, setPopupError] = useState('');
    const [copied, setCopied] = useState(false);
    const [showSource, setShowSource] = useState(false);
    const [showRenderedPopup, setShowRenderedPopup] = useState(false);

    useEffect(() => {
        return renderIntoContainer({
            containerRef,
            definition,
            postID,
            index,
            setError,
            variant: 'inline',
        });
    }, [definition, index, postID]);

    useEffect(() => {
        if (!showRenderedPopup) {
            setPopupError('');
            if (popupContainerRef.current) {
                popupContainerRef.current.innerHTML = '';
            }
            return () => undefined;
        }

        return renderIntoContainer({
            containerRef: popupContainerRef,
            definition,
            postID,
            index,
            setError: setPopupError,
            variant: 'popup',
        });
    }, [definition, index, postID, showRenderedPopup]);

    useEffect(() => {
        return () => {
            if (copyResetTimerRef.current) {
                window.clearTimeout(copyResetTimerRef.current);
            }
        };
    }, []);

    const handleCopy = async () => {
        const copySucceeded = await copyText(definition);
        if (!copySucceeded) {
            setError((currentError) => currentError || '?먮Ц 蹂듭궗???ㅽ뙣?덉뒿?덈떎. 釉뚮씪?곗? 沅뚰븳???뺤씤??二쇱꽭??');
            return;
        }

        setCopied(true);
        if (copyResetTimerRef.current) {
            window.clearTimeout(copyResetTimerRef.current);
        }
        copyResetTimerRef.current = window.setTimeout(() => {
            setCopied(false);
        }, 1600);
    };

    return (
        <>
            <div className='langflow-mermaid-card'>
                <div className='langflow-mermaid-toolbar'>
                    <button
                        className='langflow-mermaid-toolbar-button'
                        onClick={handleCopy}
                        type='button'
                    >
                        {copied ? 'Copied' : 'Copy'}
                    </button>
                    <button
                        className='langflow-mermaid-toolbar-button'
                        onClick={() => setShowSource(true)}
                        type='button'
                    >
                        {'?먮Ц 蹂닿린'}
                    </button>
                    <button
                        className='langflow-mermaid-toolbar-button'
                        onClick={() => setShowRenderedPopup(true)}
                        type='button'
                    >
                        {'?뚮뜑 ?앹뾽'}
                    </button>
                </div>
                {error && (
                    <div className='langflow-mermaid-error'>
                        {`Mermaid ?뚮뜑留??ㅽ뙣: ${error}`}
                    </div>
                )}
                {error ? (
                    <div className='langflow-mermaid-fallback'>
                        <pre className='post-code'>
                            <code className='language-mermaid'>{definition}</code>
                        </pre>
                    </div>
                ) : (
                    <div className='langflow-mermaid-rendered'>
                        <div
                            data-testid='langflow-mermaid-diagram'
                            ref={containerRef}
                        />
                    </div>
                )}
            </div>
            {showRenderedPopup && (
                <div
                    className='langflow-mermaid-modal-backdrop'
                    onClick={() => setShowRenderedPopup(false)}
                    role='presentation'
                >
                    <div
                        className='langflow-mermaid-modal langflow-mermaid-render-modal'
                        onClick={(event) => event.stopPropagation()}
                        role='dialog'
                    >
                        <div className='langflow-mermaid-modal-header'>
                            <strong>{'Mermaid ?뚮뜑 ?앹뾽'}</strong>
                            <div className='langflow-mermaid-modal-actions'>
                                <button
                                    className='langflow-mermaid-toolbar-button'
                                    onClick={handleCopy}
                                    type='button'
                                >
                                    {copied ? 'Copied' : 'Copy'}
                                </button>
                                <button
                                    className='langflow-mermaid-toolbar-button'
                                    onClick={() => setShowRenderedPopup(false)}
                                    type='button'
                                >
                                    {'?リ린'}
                                </button>
                            </div>
                        </div>
                        {popupError && (
                            <div className='langflow-mermaid-error langflow-mermaid-modal-error'>
                                {`Mermaid ?뚮뜑留??ㅽ뙣: ${popupError}`}
                            </div>
                        )}
                        <div className='langflow-mermaid-modal-content'>
                            {popupError ? (
                                <pre className='post-code langflow-mermaid-source'>
                                    <code className='language-mermaid'>{definition}</code>
                                </pre>
                            ) : (
                                <div
                                    className='langflow-mermaid-rendered langflow-mermaid-rendered-popup'
                                    data-testid='langflow-mermaid-diagram-popup'
                                    ref={popupContainerRef}
                                />
                            )}
                        </div>
                    </div>
                </div>
            )}
            {showSource && (
                <div
                    className='langflow-mermaid-modal-backdrop'
                    onClick={() => setShowSource(false)}
                    role='presentation'
                >
                    <div
                        className='langflow-mermaid-modal'
                        onClick={(event) => event.stopPropagation()}
                        role='dialog'
                    >
                        <div className='langflow-mermaid-modal-header'>
                            <strong>{'Mermaid source'}</strong>
                            <div className='langflow-mermaid-modal-actions'>
                                <button
                                    className='langflow-mermaid-toolbar-button'
                                    onClick={handleCopy}
                                    type='button'
                                >
                                    {copied ? 'Copied' : 'Copy'}
                                </button>
                                <button
                                    className='langflow-mermaid-toolbar-button'
                                    onClick={() => setShowSource(false)}
                                    type='button'
                                >
                                    {'?リ린'}
                                </button>
                            </div>
                        </div>
                        <pre className='post-code langflow-mermaid-source'>
                            <code className='language-mermaid'>{definition}</code>
                        </pre>
                    </div>
                </div>
            )}
        </>
    );
}

async function copyText(value: string) {
    try {
        if (navigator.clipboard?.writeText) {
            await navigator.clipboard.writeText(value);
            return true;
        }
    } catch {
        return legacyCopy(value);
    }

    return legacyCopy(value);
}

function legacyCopy(value: string) {
    if (typeof document === 'undefined') {
        return false;
    }

    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.setAttribute('readonly', 'true');
    textarea.style.opacity = '0';
    textarea.style.position = 'fixed';
    textarea.style.pointerEvents = 'none';
    document.body.appendChild(textarea);
    textarea.select();

    try {
        return document.execCommand('copy');
    } catch {
        return false;
    } finally {
        document.body.removeChild(textarea);
    }
}

type RenderIntoContainerOptions = {
    containerRef: React.RefObject<HTMLDivElement>;
    definition: string;
    postID: string;
    index: number;
    setError: React.Dispatch<React.SetStateAction<string>>;
    variant: string;
};

function renderIntoContainer({
    containerRef,
    definition,
    postID,
    index,
    setError,
    variant,
}: RenderIntoContainerOptions) {
    const container = containerRef.current;
    if (!container) {
        return () => undefined;
    }

    let cancelled = false;
    container.innerHTML = '';
    setError('');

    renderMermaidDefinition(definition, postID, index, variant).then(({svg, bindFunctions}) => {
        if (cancelled || !containerRef.current) {
            return;
        }
        containerRef.current.innerHTML = svg;
        bindFunctions?.(containerRef.current);
    }).catch((renderError: unknown) => {
        if (cancelled) {
            return;
        }
        const message = renderError instanceof Error ? renderError.message : String(renderError);
        setError(message);
    });

    return () => {
        cancelled = true;
        if (containerRef.current) {
            containerRef.current.innerHTML = '';
        }
    };
}
