class DBInbound {

    constructor(data) {
        this.id = 0;
        this.userId = 0;
        this.up = 0;
        this.down = 0;
        this.total = 0;
        this.allTime = 0;
        this.remark = "";
        this.enable = true;
        this.expiryTime = 0;
        this.trafficReset = "never";
        this.lastTrafficResetTime = 0;

        this.listen = "";
        this.port = 0;
        this.protocol = "";
        this.settings = "";
        this.streamSettings = "";
        this.tag = "";
        this.sniffing = "";
        this.clientStats = ""
        if (data == null) {
            return;
        }
        ObjectUtil.cloneProps(this, data);
    }

    get totalGB() {
        return NumberFormatter.toFixed(this.total / SizeFormatter.ONE_GB, 2);
    }

    set totalGB(gb) {
        this.total = NumberFormatter.toFixed(gb * SizeFormatter.ONE_GB, 0);
    }

    get isVMess() {
        return this.protocol === Protocols.VMESS;
    }

    get isVLess() {
        return this.protocol === Protocols.VLESS;
    }

    get isTrojan() {
        return this.protocol === Protocols.TROJAN;
    }

    get isSS() {
        return this.protocol === Protocols.SHADOWSOCKS;
    }

    get isMixed() {
        return this.protocol === Protocols.MIXED;
    }

    get isHTTP() {
        return this.protocol === Protocols.HTTP;
    }

    get isWireguard() {
        return this.protocol === Protocols.WIREGUARD;
    }

    get isTun() {
        return this.protocol === Protocols.TUN;
    }

    getDeprecationWarnings() {
        let settings = {};
        let stream = {};
        try { settings = JSON.parse(this.settings || '{}'); } catch (_) {}
        try { stream = JSON.parse(this.streamSettings || '{}'); } catch (_) {}
        const warnings = [];
        const add = (code, title, recommendation, severity = 'warning', meta = {}) => {
            warnings.push({ code, title, recommendation, severity, ...meta });
        };

        if (this.protocol === Protocols.VMESS) {
            add('vmess', 'VMess is deprecated', 'Convert to VLESS REALITY with Vision flow.');
        }
        if (this.protocol === Protocols.TROJAN) {
            const clients = Array.isArray(settings.clients) ? settings.clients : [];
            if (clients.length === 0 || clients.some(client => !client.flow)) {
                add('trojan-no-flow', 'Trojan without Flow is deprecated', 'Convert to VLESS REALITY/Vision. Client credentials will rotate.');
            }
        }
        if (this.protocol === Protocols.VLESS) {
            const clients = Array.isArray(settings.clients) ? settings.clients : [];
            const withoutFlow = clients.filter(client => !client.flow).length;
            const isTcpVisionTransport = stream.network === 'tcp' && ['tls', 'reality'].includes(stream.security);
            if (withoutFlow > 0 && isTcpVisionTransport) {
                add('vless-no-flow', `VLESS without Flow (${withoutFlow} client${withoutFlow === 1 ? '' : 's'})`, 'Enable xtls-rprx-vision on a TCP TLS/REALITY inbound.', 'warning', { count: withoutFlow });
            }
        }
        if (this.protocol === Protocols.SHADOWSOCKS) {
            const methods = [settings.method, ...(settings.clients || []).map(client => client.method)].filter(Boolean);
            if (methods.length === 0 || methods.some(method => !String(method).startsWith('2022-'))) {
                add('shadowsocks-legacy', 'Legacy Shadowsocks cipher', 'Move to Shadowsocks 2022 or convert to VLESS REALITY/Vision.');
            }
        }
        if (stream.network === 'grpc') {
            add('grpc-transport', 'gRPC transport is deprecated in Xray v26.9.30', 'Migrate to XHTTP stream-up over HTTP/2; v26.9.30 also has a reported reconnect regression.');
        }

        const hasLegacyAllowInsecure = value => {
            if (!value || typeof value !== 'object') return false;
            if (Object.prototype.hasOwnProperty.call(value, 'allowInsecure')) return true;
            return Object.values(value).some(child => hasLegacyAllowInsecure(child));
        };
        if (hasLegacyAllowInsecure(settings) || hasLegacyAllowInsecure(stream)) {
            add('allow-insecure', 'Legacy allowInsecure field', 'Remove allowInsecure and configure certificate pinning or name verification.', 'error');
        }
        return warnings;
    }

    get isDeprecated() {
        return this.getDeprecationWarnings().length > 0;
    }

    get address() {
        let address = location.hostname;
        if (!ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0") {
            address = this.listen;
        }
        return address;
    }

    get _expiryTime() {
        if (this.expiryTime === 0) {
            return null;
        }
        return moment(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null) {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }

    get isExpiry() {
        return this.expiryTime < new Date().getTime();
    }

    invalidateCache() {
        this._cachedInbound = null;
        this._clientStatsMap = null;
    }

    toInbound() {
        if (this._cachedInbound) {
            return this._cachedInbound;
        }

        let settings = {};
        if (!ObjectUtil.isEmpty(this.settings)) {
            settings = JSON.parse(this.settings);
        }

        let streamSettings = {};
        if (!ObjectUtil.isEmpty(this.streamSettings)) {
            streamSettings = JSON.parse(this.streamSettings);
        }

        let sniffing = {};
        if (!ObjectUtil.isEmpty(this.sniffing)) {
            sniffing = JSON.parse(this.sniffing);
        }

        const config = {
            port: this.port,
            listen: this.listen,
            protocol: this.protocol,
            settings: settings,
            streamSettings: streamSettings,
            tag: this.tag,
            sniffing: sniffing,
            clientStats: this.clientStats,
        };

        this._cachedInbound = Inbound.fromJson(config);
        return this._cachedInbound;
    }

    getClientStats(email) {
        if (!this._clientStatsMap) {
            this._clientStatsMap = new Map();
            if (this.clientStats && Array.isArray(this.clientStats)) {
                for (const stats of this.clientStats) {
                    this._clientStatsMap.set(stats.email, stats);
                }
            }
        }
        return this._clientStatsMap.get(email);
    }

    isMultiUser() {
        switch (this.protocol) {
            case Protocols.VMESS:
            case Protocols.VLESS:
            case Protocols.TROJAN:
            case Protocols.HYSTERIA:
                return true;
            case Protocols.SHADOWSOCKS:
                return this.toInbound().isSSMultiUser;
            default:
                return false;
        }
    }

    hasLink() {
        switch (this.protocol) {
            case Protocols.VMESS:
            case Protocols.VLESS:
            case Protocols.TROJAN:
            case Protocols.SHADOWSOCKS:
            case Protocols.HYSTERIA:
                return true;
            default:
                return false;
        }
    }

    genInboundLinks(remarkModel) {
        const inbound = this.toInbound();
        return inbound.genInboundLinks(this.remark, remarkModel);
    }
}
