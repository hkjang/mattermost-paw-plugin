import React from 'react';

type Props = {
    area: string;
    children: React.ReactNode;
};

type State = {
    hasError: boolean;
    message: string;
};

const containerStyle: React.CSSProperties = {
    background: 'rgba(var(--error-text-color-rgb), 0.08)',
    border: '1px solid rgba(var(--error-text-color-rgb), 0.24)',
    borderRadius: '12px',
    color: 'var(--error-text)',
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
    padding: '16px',
};

export default class PluginErrorBoundary extends React.PureComponent<Props, State> {
    public state: State = {
        hasError: false,
        message: '',
    };

    public static getDerivedStateFromError(error: Error): State {
        return {
            hasError: true,
            message: error.message || '?????녿뒗 ?ㅻ쪟媛 諛쒖깮?덉뒿?덈떎.',
        };
    }

    public componentDidCatch(error: Error, info: React.ErrorInfo) {
        // eslint-disable-next-line no-console
        console.error(`[Langflow] ${this.props.area} ?뚮뜑留??ㅻ쪟`, error, info);
    }

    public render() {
        if (this.state.hasError) {
            return (
                <div style={containerStyle}>
                    <strong>{`${this.props.area} ?붾㈃??遺덈윭?ㅼ? 紐삵뻽?듬땲??`}</strong>
                    <span>{this.state.message}</span>
                    <span style={{fontSize: '12px', opacity: 0.85}}>
                        {'?섏씠吏瑜??덈줈怨좎묠?????ㅼ떆 ?댁뼱 蹂댁꽭?? 臾몄젣媛 怨꾩냽?섎㈃ ?뚮윭洹몄씤 濡쒓렇? 釉뚮씪?곗? 肄섏넄???④퍡 ?뺤씤??二쇱꽭??'}
                    </span>
                </div>
            );
        }

        return this.props.children;
    }
}
