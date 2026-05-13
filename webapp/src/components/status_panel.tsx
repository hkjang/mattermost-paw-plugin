import React, {useEffect, useState} from 'react';

import type {ConnectionStatus, PluginStatus} from '../client';
import {getStatus, testConnection} from '../client';

const cardStyle: React.CSSProperties = {
    background: 'rgba(var(--center-channel-color-rgb), 0.04)',
    border: '1px solid rgba(var(--center-channel-color-rgb), 0.12)',
    borderRadius: '12px',
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    padding: '16px',
};

export default function StatusPanel() {
    const [status, setStatus] = useState<PluginStatus | null>(null);
    const [connection, setConnection] = useState<ConnectionStatus | null>(null);
    const [message, setMessage] = useState('');
    const [loading, setLoading] = useState(true);
    const [testing, setTesting] = useState(false);

    useEffect(() => {
        let cancelled = false;
        async function load() {
            try {
                const pluginStatus = await getStatus();
                if (!cancelled) {
                    setStatus(pluginStatus);
                }
            } catch (error) {
                if (!cancelled) {
                    setMessage((error as Error).message);
                }
            } finally {
                if (!cancelled) {
                    setLoading(false);
                }
            }
        }
        load();
        return () => {
            cancelled = true;
        };
    }, []);

    async function onTestConnection() {
        setTesting(true);
        setMessage('');
        try {
            setConnection(await testConnection());
        } catch (error) {
            setMessage((error as Error).message);
        } finally {
            setTesting(false);
        }
    }

    return (
        <div style={cardStyle}>
            <strong>{'Langflow ?곹깭'}</strong>
            {loading && <span>{'?뚮윭洹몄씤 ?곹깭瑜?遺덈윭?ㅻ뒗 以묒엯?덈떎...'}</span>}
            {!loading && status && (
                <>
                    <div>{`湲곕낯 URL: ${status.base_url || '?ㅼ젙?섏? ?딆쓬'}`}</div>
                    <div>{`?ㅼ젙??遊??? ${status.bot_count}`}</div>
                    <div>{`?덉슜 ?몄뒪?? ${(status.allow_hosts || []).join(', ') || 'Langflow ?몄뒪?몃? 湲곕낯 ?ъ슜'}`}</div>
                    <div>{`?ㅽ듃由щ컢 ?묐떟: ${status.streaming_enabled ? '?ъ슜' : '?ъ슜 ????}`}</div>
                    <div>{`?ㅽ듃由щ컢 媛깆떊 二쇨린: ${status.streaming_update_interval_ms || 0}ms`}</div>
                    {status.config_error && <div>{`?ㅼ젙 ?ㅻ쪟: ${status.config_error}`}</div>}
                    {status.bot_sync?.last_error && <div>{`遊??숆린???ㅻ쪟: ${status.bot_sync.last_error}`}</div>}
                    {(status.bots || []).length > 0 && (
                        <div style={{display: 'flex', flexDirection: 'column', gap: '10px'}}>
                            {(status.bots || []).map((bot) => {
                                const managed = (status.managed_bots || []).find((item) => item.bot_id === bot.id);
                                return (
                                    <div
                                        key={bot.id}
                                        style={{
                                            background: 'rgba(var(--center-channel-color-rgb), 0.03)',
                                            border: '1px solid rgba(var(--center-channel-color-rgb), 0.1)',
                                            borderRadius: '10px',
                                            display: 'flex',
                                            flexDirection: 'column',
                                            gap: '4px',
                                            padding: '12px',
                                        }}
                                    >
                                        <strong>{bot.display_name || bot.username}</strong>
                                        <span>{`@${bot.username} -> ${bot.flow_id}`}</span>
                                        {managed && <span>{`Mattermost ?ъ슜?? ${managed.user_id || '?앹꽦 ?湲?以?}`}</span>}
                                        {managed && <span>{`?뚮윭洹몄씤 愿由? ${managed.registered ? '?? : '?꾨땲??}, ?쒖꽦 ?곹깭: ${managed.active ? '?? : '?꾨땲??}`}</span>}
                                        {managed?.status_message && <span>{`?곹깭: ${managed.status_message}`}</span>}
                                        {bot.description && <span>{bot.description}</span>}
                                    </div>
                                );
                            })}
                        </div>
                    )}
                    <button
                        className='btn btn-primary'
                        disabled={testing}
                        onClick={onTestConnection}
                        type='button'
                    >
                        {testing ? '?곌껐 ?뺤씤 以?..' : '?곌껐 ?뚯뒪??}
                    </button>
                    {connection && (
                        <div>
                            <div>{connection.ok ? '?곌껐???깃났?덉뒿?덈떎.' : '?곌껐???ㅽ뙣?덉뒿?덈떎.'}</div>
                            <div>{connection.url}</div>
                            <div style={{whiteSpace: 'pre-wrap'}}>{connection.message}</div>
                            {connection.error_code && <div>{`?ㅻ쪟 肄붾뱶: ${connection.error_code}`}</div>}
                            {connection.detail && <div style={{whiteSpace: 'pre-wrap'}}>{`?곸꽭: ${connection.detail}`}</div>}
                            {connection.hint && <div style={{whiteSpace: 'pre-wrap'}}>{`議곗튂: ${connection.hint}`}</div>}
                            {connection.retryable !== undefined && <div>{`?ъ떆??媛?? ${connection.retryable ? '?? : '?꾨땲??}`}</div>}
                        </div>
                    )}
                </>
            )}
            {message && <span>{message}</span>}
            <div style={{fontSize: '12px', opacity: 0.8}}>
                {'System Console?먯꽌 ??ν븯硫????뚮윭洹몄씤??愿由ы븯??Mattermost 遊?怨꾩젙???앹꽦?섍굅??媛깆떊?⑸땲?? 紐⑸줉?먯꽌 ?쒓굅??遊뉗? 鍮꾪솢?깊솕?⑸땲??'}
            </div>
            <div style={{fontSize: '12px', opacity: 0.8}}>
                {'?댄썑 ?ъ슜?먮뒗 ?대떦 遊뉕낵 DM???섍굅??梨꾨꼸?먯꽌 @硫섏뀡?????덇퀬, ?뚮윭洹몄씤? 洹?遊뉗뿉 留ㅽ븨??Langflow run API瑜??몄텧?⑸땲??'}
            </div>
            <div style={{fontSize: '12px', opacity: 0.8}}>
                {'?ㅽ듃由щ컢??耳쒖졇 ?덉쑝硫?遊??듦???癒쇱? ?섎굹 ?앹꽦???? Langflow ?좏겙???꾩갑???뚮쭏??媛숈? ?ъ뒪?몃? 媛깆떊?⑸땲??'}
            </div>
        </div>
    );
}
