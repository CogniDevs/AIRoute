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
	export class RouteState {
	    is_enabled: boolean;
	    active_mode: string;
	    active_target: string;
	    active_provider: string;
	    active_ip: string;
	    latency_ms: number;
	    // Go type: time
	    last_updated: any;
	    message: string;
	
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
	        this.last_updated = this.convertValues(source["last_updated"], null);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

