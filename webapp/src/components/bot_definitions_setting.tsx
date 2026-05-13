import React, {useEffect, useMemo, useState} from 'react';

type InputFieldType = 'text' | 'textarea' | 'number' | 'bool';

type DraftInputField = {
    id: string;
    name: string;
    label: string;
    description: string;
    type: InputFieldType;
    required: boolean;
    placeholder: string;
    default_value: string | number | boolean;
};

type DraftBotDefinition = {
    local_id: string;
    id: string;
    username: string;
    display_name: string;
    description: string;
    flow_id: string;
    include_context_by_default: boolean;
    allowed_teams: string[];
    allowed_channels: string[];
    allowed_users: string[];
    input_schema: DraftInputField[];
};

type StoredInputField = {
    name?: string;
    label?: string;
    description?: string;
    type?: string;
    required?: boolean;
    placeholder?: string;
    default_value?: unknown;
};

type StoredBotDefinition = {
    id?: string;
    username?: string;
    display_name?: string;
    description?: string;
    flow_id?: string;
    include_context_by_default?: boolean;
    allowed_teams?: string[];
    allowed_channels?: string[];
    allowed_users?: string[];
    input_schema?: StoredInputField[];
};

type CustomSettingProps = {
    id?: string;
    value?: unknown;
    disabled?: boolean;
    setByEnv?: boolean;
    helpText?: React.ReactNode;
    informChange: (name: string, value: string) => void;
};

const containerStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: '16px',
};

const layoutStyle: React.CSSProperties = {
    display: 'grid',
    gap: '16px',
    gridTemplateColumns: '320px minmax(0, 1fr)',
};

const cardStyle: React.CSSProperties = {
    background: 'rgba(var(--center-channel-color-rgb), 0.04)',
    border: '1px solid rgba(var(--center-channel-color-rgb), 0.12)',
    borderRadius: '12px',
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    padding: '16px',
};

const fieldStyle: React.CSSProperties = {
    border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
    borderRadius: '8px',
    padding: '10px 12px',
    width: '100%',
};

const textAreaStyle: React.CSSProperties = {
    ...fieldStyle,
    minHeight: '96px',
    resize: 'vertical',
};

const gridTwoStyle: React.CSSProperties = {
    display: 'grid',
    gap: '12px',
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
};

const columnStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
};

const botListItemStyle = (selected: boolean): React.CSSProperties => ({
    background: selected ? 'rgba(var(--button-bg-rgb), 0.12)' : 'rgba(var(--center-channel-color-rgb), 0.03)',
    border: `1px solid ${selected ? 'rgba(var(--button-bg-rgb), 0.36)' : 'rgba(var(--center-channel-color-rgb), 0.10)'}`,
    borderRadius: '10px',
    cursor: 'pointer',
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
    padding: '12px',
    textAlign: 'left',
    width: '100%',
});

const codeStyle: React.CSSProperties = {
    background: 'rgba(var(--center-channel-color-rgb), 0.06)',
    borderRadius: '8px',
    fontFamily: 'monospace',
    fontSize: '12px',
    padding: '12px',
    whiteSpace: 'pre-wrap',
};

const sampleBots: StoredBotDefinition[] = [
    {
        id: 'thread-summary-bot',
        username: 'thread-summary-bot',
        display_name: '?ㅻ젅???붿빟 遊?,
        description: '?꾩옱 ?ㅻ젅?쒕? ?붿빟?섍퀬 ?≪뀡 ?꾩씠?쒖쓣 ?뺣━?⑸땲??',
        flow_id: 'thread-summary',
        include_context_by_default: true,
        allowed_teams: ['engineering'],
        allowed_channels: ['town-square'],
        allowed_users: [],
        input_schema: [
            {
                name: 'tone',
                label: '??,
                type: 'text',
                placeholder: '媛꾧껐?섍쾶',
                default_value: '媛꾧껐?섍쾶',
            },
        ],
    },
    {
        id: 'support-assistant-bot',
        username: 'support-assistant-bot',
        display_name: '吏???꾩슦誘?,
        description: '留ㅽ븨??Langflow flow濡?怨좉컼 吏??吏덈Ц???듬??⑸땲??',
        flow_id: 'support-assistant',
        include_context_by_default: true,
        allowed_teams: [],
        allowed_channels: [],
        allowed_users: [],
        input_schema: [],
    },
];

export default function BotDefinitionsSetting(props: CustomSettingProps) {
    const settingKey = props.id || 'BotDefinitions';
    const [bots, setBots] = useState<DraftBotDefinition[]>([]);
    const [selectedBotId, setSelectedBotId] = useState('');
    const [loadError, setLoadError] = useState('');

    useEffect(() => {
        const parsed = parseStoredBots(props.value);
        setBots(parsed.bots);
        setLoadError(parsed.error);
        setSelectedBotId((current) => {
            if (parsed.bots.length === 0) {
                return '';
            }
            if (current && parsed.bots.some((bot) => bot.local_id === current)) {
                return current;
            }
            return parsed.bots[0].local_id;
        });
    }, [props.value]);

    const selectedBot = useMemo(
        () => bots.find((bot) => bot.local_id === selectedBotId) || bots[0] || null,
        [bots, selectedBotId],
    );

    const validationMessages = useMemo(() => validateBots(bots), [bots]);
    const disabled = Boolean(props.disabled || props.setByEnv);

    const syncBots = (nextBots: DraftBotDefinition[], nextSelectedBotId?: string) => {
        setBots(nextBots);
        props.informChange(settingKey, serializeBots(nextBots));

        if (nextBots.length === 0) {
            setSelectedBotId('');
            return;
        }

        if (nextSelectedBotId) {
            setSelectedBotId(nextSelectedBotId);
            return;
        }

        setSelectedBotId((current) => {
            if (current && nextBots.some((bot) => bot.local_id === current)) {
                return current;
            }
            return nextBots[0].local_id;
        });
    };

    const updateBot = (localID: string, updater: (bot: DraftBotDefinition) => DraftBotDefinition) => {
        const nextBots = bots.map((bot) => (
            bot.local_id === localID ? updater(bot) : bot
        ));
        syncBots(nextBots, localID);
    };

    const addBot = () => {
        const bot = createEmptyBot();
        syncBots([...bots, bot], bot.local_id);
    };

    const duplicateBot = () => {
        if (!selectedBot) {
            return;
        }
        const duplicate = cloneBot(selectedBot);
        syncBots([...bots, duplicate], duplicate.local_id);
    };

    const removeSelectedBot = () => {
        if (!selectedBot) {
            return;
        }
        const nextBots = bots.filter((bot) => bot.local_id !== selectedBot.local_id);
        syncBots(nextBots);
    };

    const loadSampleBots = () => {
        const nextBots = sampleBots.map(normalizeStoredBot);
        syncBots(nextBots, nextBots[0]?.local_id);
    };

    return (
        <div style={containerStyle}>
            <section style={cardStyle}>
                <strong>{'Langflow 遊?移댄깉濡쒓렇'}</strong>
                <span style={{fontSize: '12px', opacity: 0.8}}>
                    {'?ш린?먯꽌 ?щ윭 Mattermost 遊뉗쓣 ?깅줉?????덉뒿?덈떎. 媛?遊뉗? ?섎굹??Langflow flow??怨좎젙 ?곌껐?섎ŉ DM ?먮뒗 @硫섏뀡?쇰줈 ?몄텧?⑸땲??'}
                </span>
                <span style={{fontSize: '12px', opacity: 0.8}}>
                    {'遊뉗씠 ?ㅽ뻾?섎㈃ ?뚮윭洹몄씤? POST /api/v1/run/$FLOW_ID?stream=true瑜??몄텧?섍퀬, ?꾨＼?꾪듃? 異붽? ?낅젰媛? ???而⑦뀓?ㅽ듃, Mattermost 湲곕컲 session id瑜??ы븿??JSON body瑜??꾩넚?⑸땲??'}
                </span>
                <span style={{fontSize: '12px', opacity: 0.8}}>
                    {'System Console?먯꽌 ??ν븯硫??대떦 Mattermost 遊?怨꾩젙???먮룞?쇰줈 ?앹꽦?섍굅??媛깆떊?⑸땲??'}
                </span>
                {props.helpText}
                {props.setByEnv && (
                    <span style={{color: 'var(--error-text)', fontSize: '12px'}}>
                        {'???ㅼ젙? ?섍꼍 蹂?섎줈 愿由щ릺怨??덉뼱 ?ш린?먯꽌 ?섏젙?????놁뒿?덈떎.'}
                    </span>
                )}
                {loadError && (
                    <span style={{color: 'var(--error-text)', fontSize: '12px'}}>
                        {`??λ맂 遊?移댄깉濡쒓렇瑜??댁꽍?섏? 紐삵뻽?듬땲?? ${loadError}`}
                    </span>
                )}
                {validationMessages.length > 0 && (
                    <div style={{background: 'rgba(var(--error-text-color-rgb), 0.08)', borderRadius: '8px', padding: '12px'}}>
                        <strong>{'寃利?寃곌낵'}</strong>
                        <div style={{display: 'flex', flexDirection: 'column', gap: '4px', marginTop: '8px'}}>
                            {validationMessages.map((message) => (
                                <span
                                    key={message}
                                    style={{fontSize: '12px'}}
                                >
                                    {message}
                                </span>
                            ))}
                        </div>
                    </div>
                )}
            </section>

            <div style={layoutStyle}>
                <section style={cardStyle}>
                    <div style={{display: 'flex', justifyContent: 'space-between', gap: '8px', alignItems: 'center'}}>
                        <strong>{'遊?紐⑸줉'}</strong>
                        <button
                            className='btn btn-primary'
                            disabled={disabled}
                            onClick={addBot}
                            type='button'
                        >
                            {'遊?異붽?'}
                        </button>
                    </div>

                    {bots.length === 0 && (
                        <div style={{display: 'flex', flexDirection: 'column', gap: '12px'}}>
                            <span style={{fontSize: '12px', opacity: 0.8}}>
                                {'?꾩쭅 ?ㅼ젙??遊뉗씠 ?놁뒿?덈떎. 泥?踰덉㎏ 遊뉗쓣 異붽??섍굅???덉떆 移댄깉濡쒓렇瑜?遺덈윭?ㅼ꽭??'}
                            </span>
                            <button
                                className='btn btn-secondary'
                                disabled={disabled}
                                onClick={loadSampleBots}
                                type='button'
                            >
                                {'?덉떆 遊?遺덈윭?ㅺ린'}
                            </button>
                        </div>
                    )}

                    {bots.length > 0 && (
                        <>
                            <div style={{display: 'flex', flexDirection: 'column', gap: '8px'}}>
                                {bots.map((bot) => (
                                    <button
                                        key={bot.local_id}
                                        disabled={disabled}
                                        onClick={() => setSelectedBotId(bot.local_id)}
                                        style={botListItemStyle(bot.local_id === selectedBot?.local_id)}
                                        type='button'
                                    >
                                        <strong>{bot.display_name || bot.username || '??遊?}</strong>
                                        <span style={{fontSize: '12px', opacity: 0.8}}>{`@${bot.username || 'username'}`}</span>
                                        <span style={{fontSize: '12px', opacity: 0.8}}>{bot.flow_id || '?곌껐??flow媛 ?놁뒿?덈떎'}</span>
                                    </button>
                                ))}
                            </div>
                            <div style={{display: 'flex', gap: '8px', flexWrap: 'wrap'}}>
                                <button
                                    className='btn btn-secondary'
                                    disabled={disabled || !selectedBot}
                                    onClick={duplicateBot}
                                    type='button'
                                >
                                    {'蹂듭젣'}
                                </button>
                                <button
                                    className='btn btn-secondary'
                                    disabled={disabled || !selectedBot}
                                    onClick={removeSelectedBot}
                                    type='button'
                                >
                                    {'??젣'}
                                </button>
                            </div>
                        </>
                    )}
                </section>

                <section style={cardStyle}>
                    {!selectedBot && (
                        <span style={{fontSize: '12px', opacity: 0.8}}>
                            {'遊뉗쓣 ?좏깮?섎㈃ flow ?곌껐, ?묎렐 ?뺤콉, ?낅젰 ?쇱쓣 ?섏젙?????덉뒿?덈떎.'}
                        </span>
                    )}

                    {selectedBot && (
                        <>
                            <div style={{display: 'flex', justifyContent: 'space-between', gap: '8px', alignItems: 'center'}}>
                                <strong>{selectedBot.display_name || selectedBot.username || '遊??곸꽭 ?ㅼ젙'}</strong>
                                <span style={{fontSize: '12px', opacity: 0.8}}>{`?곌껐 flow: ${selectedBot.flow_id || '$FLOW_ID'}`}</span>
                            </div>

                            <div style={gridTwoStyle}>
                                <LabeledField label={'遊?ID'}>
                                    <input
                                        disabled={disabled}
                                        onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, id: event.target.value}))}
                                        style={fieldStyle}
                                        value={selectedBot.id}
                                    />
                                </LabeledField>
                                <LabeledField label={'Flow ID'}>
                                    <input
                                        disabled={disabled}
                                        onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, flow_id: event.target.value}))}
                                        placeholder={'thread-summary'}
                                        style={fieldStyle}
                                        value={selectedBot.flow_id}
                                    />
                                </LabeledField>
                            </div>

                            <div style={gridTwoStyle}>
                                <LabeledField label={'遊??ъ슜???대쫫'}>
                                    <input
                                        disabled={disabled}
                                        onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, username: sanitizeUsername(event.target.value)}))}
                                        placeholder={'thread-summary-bot'}
                                        style={fieldStyle}
                                        value={selectedBot.username}
                                    />
                                </LabeledField>
                                <LabeledField label={'?쒖떆 ?대쫫'}>
                                    <input
                                        disabled={disabled}
                                        onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, display_name: event.target.value}))}
                                        placeholder={'?ㅻ젅???붿빟 遊?}
                                        style={fieldStyle}
                                        value={selectedBot.display_name}
                                    />
                                </LabeledField>
                            </div>

                            <LabeledField label={'?ㅻ챸'}>
                                <textarea
                                    disabled={disabled}
                                    onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, description: event.target.value}))}
                                    placeholder={'??遊뉗씠 Mattermost?먯꽌 臾댁뾿???섎뒗吏 ?ㅻ챸?섏꽭??'}
                                    style={textAreaStyle}
                                    value={selectedBot.description}
                                />
                            </LabeledField>

                            <label style={{display: 'flex', gap: '8px', alignItems: 'center'}}>
                                <input
                                    checked={selectedBot.include_context_by_default}
                                    disabled={disabled}
                                    onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, include_context_by_default: event.target.checked}))}
                                    type='checkbox'
                                />
                                {'理쒓렐 Mattermost ??붾? 湲곕낯 而⑦뀓?ㅽ듃濡??ы븿'}
                            </label>

                            <div style={gridTwoStyle}>
                                <LabeledField label={'?덉슜 ?'}>
                                    <input
                                        disabled={disabled}
                                        onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, allowed_teams: splitCSV(event.target.value)}))}
                                        placeholder={'team-name, team-id'}
                                        style={fieldStyle}
                                        value={joinCSV(selectedBot.allowed_teams)}
                                    />
                                </LabeledField>
                                <LabeledField label={'?덉슜 梨꾨꼸'}>
                                    <input
                                        disabled={disabled}
                                        onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, allowed_channels: splitCSV(event.target.value)}))}
                                        placeholder={'town-square, channel-id'}
                                        style={fieldStyle}
                                        value={joinCSV(selectedBot.allowed_channels)}
                                    />
                                </LabeledField>
                            </div>

                            <LabeledField label={'?덉슜 ?ъ슜??}>
                                <input
                                    disabled={disabled}
                                    onChange={(event) => updateBot(selectedBot.local_id, (bot) => ({...bot, allowed_users: splitCSV(event.target.value)}))}
                                    placeholder={'sysadmin, user-id'}
                                    style={fieldStyle}
                                    value={joinCSV(selectedBot.allowed_users)}
                                />
                            </LabeledField>

                            <section style={{...cardStyle, padding: '12px'}}>
                                <div style={{display: 'flex', justifyContent: 'space-between', gap: '8px', alignItems: 'center'}}>
                                    <strong>{'異붽? ?낅젰 ?꾨뱶'}</strong>
                                    <button
                                        className='btn btn-secondary'
                                        disabled={disabled}
                                        onClick={() => updateBot(selectedBot.local_id, (bot) => ({...bot, input_schema: [...bot.input_schema, createEmptyInputField()]}))}
                                        type='button'
                                    >
                                        {'?꾨뱶 異붽?'}
                                    </button>
                                </div>

                                {selectedBot.input_schema.length === 0 && (
                                    <span style={{fontSize: '12px', opacity: 0.8}}>
                                        {'異붽? ?낅젰 ?꾨뱶媛 ?놁뒿?덈떎. ??寃쎌슦 硫붿씤 ?꾨＼?꾪듃留?Langflow濡??꾩넚?⑸땲??'}
                                    </span>
                                )}

                                {selectedBot.input_schema.map((field, index) => (
                                    <div
                                        key={field.id}
                                        style={{border: '1px solid rgba(var(--center-channel-color-rgb), 0.1)', borderRadius: '10px', padding: '12px'}}
                                    >
                                        <div style={{display: 'flex', justifyContent: 'space-between', gap: '8px', alignItems: 'center'}}>
                                            <strong>{field.label || field.name || `?꾨뱶 ${index + 1}`}</strong>
                                            <button
                                                className='btn btn-secondary'
                                                disabled={disabled}
                                                onClick={() => updateBot(selectedBot.local_id, (bot) => ({
                                                    ...bot,
                                                    input_schema: bot.input_schema.filter((item) => item.id !== field.id),
                                                }))}
                                                type='button'
                                            >
                                                {'??젣'}
                                            </button>
                                        </div>

                                        <div style={{...gridTwoStyle, marginTop: '12px'}}>
                                            <LabeledField label={'?꾨뱶 ?대쫫'}>
                                                <input
                                                    disabled={disabled}
                                                    onChange={(event) => updateInputField(selectedBot.local_id, field.id, {name: event.target.value}, updateBot)}
                                                    placeholder={'tone'}
                                                    style={fieldStyle}
                                                    value={field.name}
                                                />
                                            </LabeledField>
                                            <LabeledField label={'?쒖떆 ?쇰꺼'}>
                                                <input
                                                    disabled={disabled}
                                                    onChange={(event) => updateInputField(selectedBot.local_id, field.id, {label: event.target.value}, updateBot)}
                                                    placeholder={'??}
                                                    style={fieldStyle}
                                                    value={field.label}
                                                />
                                            </LabeledField>
                                        </div>

                                        <div style={{...gridTwoStyle, marginTop: '12px'}}>
                                            <LabeledField label={'???}>
                                                <select
                                                    disabled={disabled}
                                                    onChange={(event) => updateInputField(selectedBot.local_id, field.id, {type: event.target.value as InputFieldType, default_value: defaultValueForType(event.target.value as InputFieldType)}, updateBot)}
                                                    style={fieldStyle}
                                                    value={field.type}
                                                >
                                                    <option value='text'>{'?띿뒪??}</option>
                                                    <option value='textarea'>{'?щ윭 以??띿뒪??}</option>
                                                    <option value='number'>{'?レ옄'}</option>
                                                    <option value='bool'>{'遺덈━??}</option>
                                                </select>
                                            </LabeledField>
                                            <LabeledField label={'?뚮젅?댁뒪???}>
                                                <input
                                                    disabled={disabled}
                                                    onChange={(event) => updateInputField(selectedBot.local_id, field.id, {placeholder: event.target.value}, updateBot)}
                                                    placeholder={'媛꾧껐?섍쾶'}
                                                    style={fieldStyle}
                                                    value={field.placeholder}
                                                />
                                            </LabeledField>
                                        </div>

                                        <LabeledField label={'?ㅻ챸'}>
                                            <input
                                                disabled={disabled}
                                                onChange={(event) => updateInputField(selectedBot.local_id, field.id, {description: event.target.value}, updateBot)}
                                                placeholder={'?ъ슜?먯뿉寃?蹂댁뿬 以??덈궡 臾멸뎄?낅땲??'}
                                                style={fieldStyle}
                                                value={field.description}
                                            />
                                        </LabeledField>

                                        <div style={{...gridTwoStyle, marginTop: '12px'}}>
                                            <LabeledField label={'湲곕낯媛?}>
                                                {renderDefaultValueEditor(field, disabled, (value) => updateInputField(selectedBot.local_id, field.id, {default_value: value}, updateBot))}
                                            </LabeledField>
                                            <div style={columnStyle}>
                                                <span style={{fontWeight: 600}}>{'?꾩닔 ?щ?'}</span>
                                                <label style={{display: 'flex', gap: '8px', alignItems: 'center'}}>
                                                    <input
                                                        checked={field.required}
                                                        disabled={disabled}
                                                        onChange={(event) => updateInputField(selectedBot.local_id, field.id, {required: event.target.checked}, updateBot)}
                                                        type='checkbox'
                                                    />
                                                    {'?ㅽ뻾 ?꾩뿉 ?ъ슜?먭? 諛섎뱶???낅젰?댁빞 ?⑸땲??}
                                                </label>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </section>

                            <section style={{...cardStyle, padding: '12px'}}>
                                <strong>{'?몄텧 誘몃━蹂닿린'}</strong>
                                <div style={codeStyle}>
                                    {buildCurlPreview(selectedBot)}
                                </div>
                                <span style={{fontSize: '12px', opacity: 0.8}}>
                                    {selectedBot.username ? `?ъ슜?먮뒗 梨꾨꼸, ?ㅻ젅?? DM?먯꽌 @${selectedBot.username} ?뺥깭濡???遊뉗쓣 ?몄텧?????덉뒿?덈떎.` : '硫섏뀡 諛?DM ?쇱슦?낆쓣 ?ъ슜?섎젮硫?遊?username???ㅼ젙?섏꽭??'}
                                </span>
                            </section>
                        </>
                    )}
                </section>
            </div>

            <details style={cardStyle}>
                <summary style={{cursor: 'pointer', fontWeight: 600}}>{'怨좉툒 JSON 誘몃━蹂닿린'}</summary>
                <pre style={codeStyle}>{serializeBots(bots)}</pre>
            </details>
        </div>
    );
}

function LabeledField(props: {label: string; children: React.ReactNode}) {
    return (
        <div style={columnStyle}>
            <span style={{fontWeight: 600}}>{props.label}</span>
            {props.children}
        </div>
    );
}

function renderDefaultValueEditor(field: DraftInputField, disabled: boolean, onChange: (value: string | number | boolean) => void) {
    if (field.type === 'bool') {
        return (
            <label style={{display: 'flex', gap: '8px', alignItems: 'center'}}>
                <input
                    checked={Boolean(field.default_value)}
                    disabled={disabled}
                    onChange={(event) => onChange(event.target.checked)}
                    type='checkbox'
                />
                {'湲곕낯媛믪쑝濡?泥댄겕??}
            </label>
        );
    }

    if (field.type === 'number') {
        return (
            <input
                disabled={disabled}
                onChange={(event) => onChange(Number(event.target.value || 0))}
                style={fieldStyle}
                type='number'
                value={String(field.default_value)}
            />
        );
    }

    return (
        <input
            disabled={disabled}
            onChange={(event) => onChange(event.target.value)}
            style={fieldStyle}
            type='text'
            value={String(field.default_value)}
        />
    );
}

function updateInputField(
    botLocalID: string,
    fieldID: string,
    patch: Partial<DraftInputField>,
    updateBot: (localID: string, updater: (bot: DraftBotDefinition) => DraftBotDefinition) => void,
) {
    updateBot(botLocalID, (bot) => ({
        ...bot,
        input_schema: bot.input_schema.map((field) => (
            field.id === fieldID ? {...field, ...patch} : field
        )),
    }));
}

function parseStoredBots(rawValue: unknown) {
    if (!rawValue) {
        return {bots: [] as DraftBotDefinition[], error: ''};
    }

    try {
        const parsed = typeof rawValue === 'string' ? JSON.parse(rawValue || '[]') : rawValue;
        if (!Array.isArray(parsed)) {
            return {bots: [] as DraftBotDefinition[], error: '??λ맂 媛믪씠 JSON 諛곗뿴 ?뺤떇???꾨떃?덈떎.'};
        }
        return {
            bots: parsed.map((item) => normalizeStoredBot(item as StoredBotDefinition)),
            error: '',
        };
    } catch (error) {
        return {
            bots: [] as DraftBotDefinition[],
            error: (error as Error).message,
        };
    }
}

function normalizeStoredBot(value: StoredBotDefinition): DraftBotDefinition {
    return {
        local_id: createLocalID('bot'),
        id: stringValue(value.id),
        username: sanitizeUsername(stringValue(value.username)),
        display_name: stringValue(value.display_name),
        description: stringValue(value.description),
        flow_id: stringValue(value.flow_id),
        include_context_by_default: Boolean(value.include_context_by_default),
        allowed_teams: normalizeStringArray(value.allowed_teams),
        allowed_channels: normalizeStringArray(value.allowed_channels),
        allowed_users: normalizeStringArray(value.allowed_users),
        input_schema: Array.isArray(value.input_schema) ? value.input_schema.map(normalizeInputField) : [],
    };
}

function normalizeInputField(value: StoredInputField): DraftInputField {
    const type = normalizeInputType(stringValue(value?.type));
    return {
        id: createLocalID('input'),
        name: stringValue(value?.name),
        label: stringValue(value?.label),
        description: stringValue(value?.description),
        type,
        required: Boolean(value?.required),
        placeholder: stringValue(value?.placeholder),
        default_value: normalizeDefaultValue(type, value?.default_value),
    };
}

function serializeBots(bots: DraftBotDefinition[]) {
    return JSON.stringify(bots.map((bot) => ({
        id: bot.id.trim(),
        username: bot.username.trim(),
        display_name: bot.display_name.trim(),
        description: bot.description.trim(),
        flow_id: bot.flow_id.trim(),
        include_context_by_default: bot.include_context_by_default,
        allowed_teams: normalizeStringArray(bot.allowed_teams),
        allowed_channels: normalizeStringArray(bot.allowed_channels),
        allowed_users: normalizeStringArray(bot.allowed_users),
        input_schema: bot.input_schema.map((field) => ({
            name: field.name.trim(),
            label: field.label.trim(),
            description: field.description.trim(),
            type: field.type,
            required: field.required,
            placeholder: field.placeholder.trim(),
            default_value: field.default_value,
        })),
    })), null, 2);
}

function validateBots(bots: DraftBotDefinition[]) {
    const messages: string[] = [];
    const seenIDs = new Set<string>();
    const seenUsernames = new Set<string>();

    bots.forEach((bot, index) => {
        const label = bot.display_name || bot.username || `遊?${index + 1}`;
        const botID = bot.id.trim();
        const username = bot.username.trim();
        const flowID = bot.flow_id.trim();

        if (!botID) {
            messages.push(`${label}: 遊?ID???꾩닔?낅땲??`);
        } else if (seenIDs.has(botID)) {
            messages.push(`${label}: 遊?ID "${botID}"媛 以묐났?섏뿀?듬땲??`);
        } else {
            seenIDs.add(botID);
        }

        if (!username) {
            messages.push(`${label}: 遊?username? ?꾩닔?낅땲??`);
        } else if (seenUsernames.has(username)) {
            messages.push(`${label}: 遊?username "${username}"??以묐났?섏뿀?듬땲??`);
        } else {
            seenUsernames.add(username);
        }

        if (!bot.display_name.trim()) {
            messages.push(`${label}: ?쒖떆 ?대쫫? ?꾩닔?낅땲??`);
        }

        if (!flowID) {
            messages.push(`${label}: flow ID???꾩닔?낅땲??`);
        }

        const seenFields = new Set<string>();
        bot.input_schema.forEach((field, fieldIndex) => {
            const fieldLabel = field.label || field.name || `?꾨뱶 ${fieldIndex + 1}`;
            const fieldName = field.name.trim();
            if (!fieldName) {
                messages.push(`${label}: ${fieldLabel}???꾨뱶 ?대쫫???놁뒿?덈떎.`);
            } else if (seenFields.has(fieldName)) {
                messages.push(`${label}: ?낅젰 ?꾨뱶 "${fieldName}"媛 以묐났?섏뿀?듬땲??`);
            } else {
                seenFields.add(fieldName);
            }
        });
    });

    return messages;
}

function buildCurlPreview(bot: DraftBotDefinition) {
    const flowID = bot.flow_id || '$FLOW_ID';
    const username = bot.username || 'bot-username';
    return [
        `curl -X POST "$LANGFLOW_BASE_URL/api/v1/run/${flowID}?stream=true" \\`,
        '  -H "Authorization: Bearer $LANGFLOW_API_KEY" \\',
        '  -H "Content-Type: application/json" \\',
        "  -d '{",
        `    "input_value": "Hello from @${username}",`,
        '    "session_id": "mattermost:bot-id:thread-or-channel:user-id"',
        "  }'",
    ].join('\n');
}

function createEmptyBot(): DraftBotDefinition {
    return {
        local_id: createLocalID('bot'),
        id: '',
        username: '',
        display_name: '',
        description: '',
        flow_id: '',
        include_context_by_default: true,
        allowed_teams: [],
        allowed_channels: [],
        allowed_users: [],
        input_schema: [],
    };
}

function cloneBot(bot: DraftBotDefinition): DraftBotDefinition {
    return {
        ...bot,
        local_id: createLocalID('bot'),
        id: bot.id ? `${bot.id}-copy` : '',
        username: bot.username ? `${bot.username}-copy` : '',
        display_name: bot.display_name ? `${bot.display_name} Copy` : '',
        input_schema: bot.input_schema.map((field) => ({
            ...field,
            id: createLocalID('input'),
        })),
    };
}

function createEmptyInputField(): DraftInputField {
    return {
        id: createLocalID('input'),
        name: '',
        label: '',
        description: '',
        type: 'text',
        required: false,
        placeholder: '',
        default_value: '',
    };
}

function normalizeInputType(value: string): InputFieldType {
    if (value === 'textarea' || value === 'number' || value === 'bool') {
        return value;
    }
    return 'text';
}

function normalizeDefaultValue(type: InputFieldType, value: unknown) {
    if (type === 'bool') {
        return Boolean(value);
    }
    if (type === 'number') {
        return typeof value === 'number' ? value : Number(value || 0);
    }
    return stringValue(value);
}

function defaultValueForType(type: InputFieldType) {
    if (type === 'bool') {
        return false;
    }
    if (type === 'number') {
        return 0;
    }
    return '';
}

function splitCSV(value: string) {
    return normalizeStringArray(value.split(','));
}

function joinCSV(values: string[]) {
    return normalizeStringArray(values).join(', ');
}

function normalizeStringArray(values: unknown) {
    if (!Array.isArray(values)) {
        return [];
    }

    return values.map((item) => stringValue(item).trim()).filter(Boolean);
}

function stringValue(value: unknown) {
    if (typeof value === 'string') {
        return value;
    }
    if (value == null) {
        return '';
    }
    return String(value);
}

function sanitizeUsername(value: string) {
    return value.toLowerCase().replace(/[^a-z0-9-_]/g, '');
}

function createLocalID(prefix: string) {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
        return `${prefix}-${crypto.randomUUID()}`;
    }

    return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
