export namespace domain {
	
	export class ProbeResult {
	    provider_id: string;
	    provider_name: string;
	    ip: string;
	    latency_ms: number;
	    is_clean: boolean;
	    reason: string;
	    target: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider_id = source["provider_id"];
	        this.provider_name = source["provider_name"];
	        this.ip = source["ip"];
	        this.latency_ms = source["latency_ms"];
	        this.is_clean = source["is_clean"];
	        this.reason = source["reason"];
	        this.target = source["target"];
	    }
	}

	export class Provider {
	    id: string;
	    name: string;
	    kind: string;
	    endpoint: string;
	    fallback_ips?: string[];
	    gateways?: string[];
	    enabled: boolean;
	    is_custom: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Provider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.endpoint = source["endpoint"];
	        this.fallback_ips = source["fallback_ips"];
	        this.gateways = source["gateways"];
	        this.enabled = source["enabled"];
	        this.is_custom = source["is_custom"];
	    }
	}

	export class ServiceRoute {
	    target: string;
	    enabled: boolean;
	    provider_id: string;
	    provider_name: string;
	    ip: string;
	    latency_ms: number;

	    static createFrom(source: any = {}) {
	        return new ServiceRoute(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.enabled = source["enabled"];
	        this.provider_id = source["provider_id"];
	        this.provider_name = source["provider_name"];
	        this.ip = source["ip"];
	        this.latency_ms = source["latency_ms"];
	    }
	}

	export class RouteState {
	    is_enabled: boolean;
	    active_mode: string;
	    active_target: string;
	    active_provider: string;
	    active_ip: string;
	    latency_ms: number;
	    last_updated: any;
	    message: string;
	    routes: { [key: string]: ServiceRoute };
	    patched_count: number;
	    total_count: number;
	    hosts_path: string;
	    proxy_running: boolean;
	    proxy_addr: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_enabled = source["is_enabled"];
	        this.active_mode = source["active_mode"];
	        this.active_target = source["active_target"];
	        this.active_provider = source["active_provider"];
	        this.active_ip = source["active_ip"];
	        this.latency_ms = source["latency_ms"];
	        this.last_updated = source["last_updated"];
	        this.message = source["message"];
	        this.routes = source["routes"] || {};
	        this.patched_count = source["patched_count"] || 0;
	        this.total_count = source["total_count"] || 0;
	        this.hosts_path = source["hosts_path"] || '';
	        this.proxy_running = source["proxy_running"] || false;
	        this.proxy_addr = source["proxy_addr"] || '';
	    }
	}

}
