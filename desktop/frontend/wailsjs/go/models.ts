export namespace main {
	
	export class accountView {
	    name: string;
	    currency: string;
	
	    static createFrom(source: any = {}) {
	        return new accountView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.currency = source["currency"];
	    }
	}
	export class findingView {
	    severity: string;
	    kind: string;
	    account: string;
	    transferAccount: string;
	    time: string;
	    amount: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new findingView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.kind = source["kind"];
	        this.account = source["account"];
	        this.transferAccount = source["transferAccount"];
	        this.time = source["time"];
	        this.amount = source["amount"];
	        this.message = source["message"];
	    }
	}
	export class transactionView {
	    time: string;
	    type: string;
	    account: string;
	    currency: string;
	    amount: number;
	    transferAccount: string;
	    transferAmount: number;
	    category: string;
	    subCategory: string;
	    memo: string;
	
	    static createFrom(source: any = {}) {
	        return new transactionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.type = source["type"];
	        this.account = source["account"];
	        this.currency = source["currency"];
	        this.amount = source["amount"];
	        this.transferAccount = source["transferAccount"];
	        this.transferAmount = source["transferAmount"];
	        this.category = source["category"];
	        this.subCategory = source["subCategory"];
	        this.memo = source["memo"];
	    }
	}
	export class fileInfoView {
	    path: string;
	    name: string;
	    size: number;
	    sha256: string;
	
	    static createFrom(source: any = {}) {
	        return new fileInfoView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class extractionView {
	    file: fileInfoView;
	    accounts: accountView[];
	    transactions: transactionView[];
	    skipped: number;
	    diagnostics: msmoney.ExtractedDiagnostics;
	    validation: findingView[];
	    elapsed: string;
	
	    static createFrom(source: any = {}) {
	        return new extractionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = this.convertValues(source["file"], fileInfoView);
	        this.accounts = this.convertValues(source["accounts"], accountView);
	        this.transactions = this.convertValues(source["transactions"], transactionView);
	        this.skipped = source["skipped"];
	        this.diagnostics = this.convertValues(source["diagnostics"], msmoney.ExtractedDiagnostics);
	        this.validation = this.convertValues(source["validation"], findingView);
	        this.elapsed = source["elapsed"];
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

export namespace msmoney {
	
	export class ExtractedDiagnostics {
	    CorruptRows: number;
	    UnresolvedRows: number;
	    CorruptDefinitions: number;
	    UnknownCatalogEntries: number;
	    Samples: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExtractedDiagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CorruptRows = source["CorruptRows"];
	        this.UnresolvedRows = source["UnresolvedRows"];
	        this.CorruptDefinitions = source["CorruptDefinitions"];
	        this.UnknownCatalogEntries = source["UnknownCatalogEntries"];
	        this.Samples = source["Samples"];
	    }
	}

}

