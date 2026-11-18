import {domain} from '../models';

export function AddCustomProvider(arg1:domain.Provider):Promise<void>;

export function ApplyAutoBest(enabledTargets:Array<string>):Promise<domain.RouteState>;

export function ApplyRoutes(selections:{[key: string]: string}, enabledTargets:Array<string>):Promise<domain.RouteState>;

export function ClearRoutes():Promise<domain.RouteState>;

export function GetHostsPath():Promise<string>;

export function GetProviders():Promise<Array<domain.Provider>>;

export function GetStatus():Promise<domain.RouteState>;

export function RemoveCustomProvider(arg1:string):Promise<void>;

export function TestProviders(arg1:domain.ServiceTarget):Promise<Array<domain.ProbeResult>>;

export function ToggleProxy(enable:boolean, port:number):Promise<domain.RouteState>;

export function ToggleRoute(arg1:boolean,arg2:domain.ServiceTarget,arg3:string):Promise<domain.RouteState>;

export function VerifyActive():Promise<Array<domain.ProbeResult>>;
