export namespace api {
	
	export class BackupProgress {
	    job_id: string;
	    running: boolean;
	    progress: number;
	    message: string;
	    success: boolean;
	    complete: boolean;
	    error?: string;
	    start_time?: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.running = source["running"];
	        this.progress = source["progress"];
	        this.message = source["message"];
	        this.success = source["success"];
	        this.complete = source["complete"];
	        this.error = source["error"];
	        this.start_time = source["start_time"];
	    }
	}
	export class PBSTicket {
	    ticket: string;
	    csrf_token?: string;
	    base_url: string;
	    cert_fingerprint?: string;
	    datastore?: string;
	    namespace?: string;
	
	    static createFrom(source: any = {}) {
	        return new PBSTicket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ticket = source["ticket"];
	        this.csrf_token = source["csrf_token"];
	        this.base_url = source["base_url"];
	        this.cert_fingerprint = source["cert_fingerprint"];
	        this.datastore = source["datastore"];
	        this.namespace = source["namespace"];
	    }
	}

}

export namespace main {
	
	export class BackupMeta {
	    backup_id: string;
	    original_path: string;
	    hostname: string;
	    backup_time: string;
	    client_version: string;
	    os: string;
	    vss_used: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BackupMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backup_id = source["backup_id"];
	        this.original_path = source["original_path"];
	        this.hostname = source["hostname"];
	        this.backup_time = source["backup_time"];
	        this.client_version = source["client_version"];
	        this.os = source["os"];
	        this.vss_used = source["vss_used"];
	    }
	}
	export class Brand {
	    name: string;
	    title: string;
	    logo: string;
	    accent: string;
	    accent_hover: string;
	    brand_url: string;
	    buy_storage_url: string;
	    buy_storage_text: string;
	    urls: Record<string, string>;
	    is_default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Brand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.title = source["title"];
	        this.logo = source["logo"];
	        this.accent = source["accent"];
	        this.accent_hover = source["accent_hover"];
	        this.brand_url = source["brand_url"];
	        this.buy_storage_url = source["buy_storage_url"];
	        this.buy_storage_text = source["buy_storage_text"];
	        this.urls = source["urls"];
	        this.is_default = source["is_default"];
	    }
	}
	export class PBSServer {
	    id: string;
	    name: string;
	    baseurl: string;
	    certfingerprint: string;
	    authid: string;
	    secret: string;
	    username?: string;
	    password?: string;
	    encryption_key_file?: string;
	    datastore: string;
	    namespace: string;
	    description?: string;
	    is_online?: boolean;
	    secret_set?: boolean;
	    password_set?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PBSServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseurl = source["baseurl"];
	        this.certfingerprint = source["certfingerprint"];
	        this.authid = source["authid"];
	        this.secret = source["secret"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.encryption_key_file = source["encryption_key_file"];
	        this.datastore = source["datastore"];
	        this.namespace = source["namespace"];
	        this.description = source["description"];
	        this.is_online = source["is_online"];
	        this.secret_set = source["secret_set"];
	        this.password_set = source["password_set"];
	    }
	}
	export class Config {
	    pbs_servers?: Record<string, PBSServer>;
	    default_pbs_id?: string;
	    baseurl?: string;
	    certfingerprint?: string;
	    authid?: string;
	    secret?: string;
	    datastore?: string;
	    namespace?: string;
	    encryption_key_file?: string;
	    backupdir?: string;
	    "backup-id"?: string;
	    usevss: boolean;
	    last_backup_dirs?: string[];
	    disable_split?: boolean;
	    split_size_gb?: number;
	    smtp_host?: string;
	    smtp_port?: string;
	    smtp_username?: string;
	    smtp_password?: string;
	    email_from?: string;
	    email_to?: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pbs_servers = this.convertValues(source["pbs_servers"], PBSServer, true);
	        this.default_pbs_id = source["default_pbs_id"];
	        this.baseurl = source["baseurl"];
	        this.certfingerprint = source["certfingerprint"];
	        this.authid = source["authid"];
	        this.secret = source["secret"];
	        this.datastore = source["datastore"];
	        this.namespace = source["namespace"];
	        this.encryption_key_file = source["encryption_key_file"];
	        this.backupdir = source["backupdir"];
	        this["backup-id"] = source["backup-id"];
	        this.usevss = source["usevss"];
	        this.last_backup_dirs = source["last_backup_dirs"];
	        this.disable_split = source["disable_split"];
	        this.split_size_gb = source["split_size_gb"];
	        this.smtp_host = source["smtp_host"];
	        this.smtp_port = source["smtp_port"];
	        this.smtp_username = source["smtp_username"];
	        this.smtp_password = source["smtp_password"];
	        this.email_from = source["email_from"];
	        this.email_to = source["email_to"];
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
	export class EncryptionKeyInfo {
	    path: string;
	    exists: boolean;
	    fingerprint: string;
	    created?: string;
	    modified?: string;
	    hint?: string;
	    passphrase_protected: boolean;
	    usable: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new EncryptionKeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.fingerprint = source["fingerprint"];
	        this.created = source["created"];
	        this.modified = source["modified"];
	        this.hint = source["hint"];
	        this.passphrase_protected = source["passphrase_protected"];
	        this.usable = source["usable"];
	        this.reason = source["reason"];
	    }
	}
	export class JobHistory {
	    id: string;
	    name: string;
	    timestamp: string;
	    status: string;
	    message: string;
	    backupDirs: string[];
	    backupId: string;
	    useVSS: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JobHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.timestamp = source["timestamp"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.backupDirs = source["backupDirs"];
	        this.backupId = source["backupId"];
	        this.useVSS = source["useVSS"];
	    }
	}
	
	export class PhysicalDiskInfo {
	    disk_number: number;
	    size: number;
	    model: string;
	    is_boot_disk: boolean;
	    is_system_disk: boolean;
	    device_id: string;
	    device_path: string;
	
	    static createFrom(source: any = {}) {
	        return new PhysicalDiskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.disk_number = source["disk_number"];
	        this.size = source["size"];
	        this.model = source["model"];
	        this.is_boot_disk = source["is_boot_disk"];
	        this.is_system_disk = source["is_system_disk"];
	        this.device_id = source["device_id"];
	        this.device_path = source["device_path"];
	    }
	}
	export class ScheduledJob {
	    id: string;
	    name: string;
	    scheduleTime: string;
	    runAtStartup: boolean;
	    backupDirs: string[];
	    driveLetters: string[];
	    backupId: string;
	    useVSS: boolean;
	    backupType: string;
	    excludeList: string[];
	    compression: string;
	    pbs_id?: string;
	    backup_kind?: string;
	    lastRun?: string;
	    nextRun?: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScheduledJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scheduleTime = source["scheduleTime"];
	        this.runAtStartup = source["runAtStartup"];
	        this.backupDirs = source["backupDirs"];
	        this.driveLetters = source["driveLetters"];
	        this.backupId = source["backupId"];
	        this.useVSS = source["useVSS"];
	        this.backupType = source["backupType"];
	        this.excludeList = source["excludeList"];
	        this.compression = source["compression"];
	        this.pbs_id = source["pbs_id"];
	        this.backup_kind = source["backup_kind"];
	        this.lastRun = source["lastRun"];
	        this.nextRun = source["nextRun"];
	        this.enabled = source["enabled"];
	    }
	}
	export class SearchHit {
	    backup_id: string;
	    snapshot_time: number;
	    path: string;
	    origin_path: string;
	    is_dir: boolean;
	    size: number;
	    mtime: number;
	    from_cache: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backup_id = source["backup_id"];
	        this.snapshot_time = source["snapshot_time"];
	        this.path = source["path"];
	        this.origin_path = source["origin_path"];
	        this.is_dir = source["is_dir"];
	        this.size = source["size"];
	        this.mtime = source["mtime"];
	        this.from_cache = source["from_cache"];
	    }
	}
	export class SearchResult {
	    hits: SearchHit[];
	    snapshots_in_range: number;
	    snapshots_searched: number;
	    snapshots_skipped: number;
	    snapshots_assembled: number;
	    truncated: boolean;
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hits = this.convertValues(source["hits"], SearchHit);
	        this.snapshots_in_range = source["snapshots_in_range"];
	        this.snapshots_searched = source["snapshots_searched"];
	        this.snapshots_skipped = source["snapshots_skipped"];
	        this.snapshots_assembled = source["snapshots_assembled"];
	        this.truncated = source["truncated"];
	        this.cancelled = source["cancelled"];
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
	export class SnapshotEntry {
	    path: string;
	    is_dir: boolean;
	    size: number;
	    mtime: number;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.is_dir = source["is_dir"];
	        this.size = source["size"];
	        this.mtime = source["mtime"];
	    }
	}

}

