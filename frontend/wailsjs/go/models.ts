export namespace main {
	
	export class AudioDeviceInfo {
	    id: string;
	    name: string;
	    isDefault: boolean;
	    isChosen: boolean;
	    isPending: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AudioDeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.isDefault = source["isDefault"];
	        this.isChosen = source["isChosen"];
	        this.isPending = source["isPending"];
	    }
	}

}

