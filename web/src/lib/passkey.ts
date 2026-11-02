import { api } from './api.ts';
import type { LoginResult, Passkey } from './types.ts';

type Ceremony<T> = { ticket: string; options: T };

export function passkeysSupported() {
	return typeof PublicKeyCredential !== 'undefined' && 'parseRequestOptionsFromJSON' in PublicKeyCredential;
}

export function passkeyCancelled(err: unknown) {
	return err instanceof DOMException && (err.name === 'NotAllowedError' || err.name === 'AbortError');
}

export async function signInWithPasskey() {
	const { ticket, options } = await api.post<Ceremony<PublicKeyCredentialRequestOptionsJSON>>('/auth/passkey/begin');
	const publicKey = PublicKeyCredential.parseRequestOptionsFromJSON(options);
	const credential = (await navigator.credentials.get({ publicKey })) as PublicKeyCredential | null;
	if (!credential) throw new DOMException('cancelled', 'NotAllowedError');
	return api.post<LoginResult>('/auth/passkey/finish', { ticket, credential: credential.toJSON() });
}

export async function registerPasskey(name: string) {
	const { ticket, options } = await api.post<Ceremony<PublicKeyCredentialCreationOptionsJSON>>(
		'/admin/account/passkeys/begin'
	);
	const publicKey = PublicKeyCredential.parseCreationOptionsFromJSON(options);
	const credential = (await navigator.credentials.create({ publicKey })) as PublicKeyCredential | null;
	if (!credential) throw new DOMException('cancelled', 'NotAllowedError');
	return api.post<Passkey>('/admin/account/passkeys/finish', { ticket, name, credential: credential.toJSON() });
}

const systems = [
	['iPhone', 'iPhone'],
	['iPad', 'iPad'],
	['Android', 'Android'],
	['Mac OS X', 'Mac'],
	['Windows', 'Windows'],
	['Linux', 'Linux']
] as const;

const browsers = [
	['Edg/', 'Edge'],
	['Firefox/', 'Firefox'],
	['Chrome/', 'Chrome'],
	['Safari/', 'Safari']
] as const;

export function deviceName() {
	const ua = navigator.userAgent;
	const os = systems.find(([needle]) => ua.includes(needle))?.[1];
	const browser = browsers.find(([needle]) => ua.includes(needle))?.[1];
	return [browser, os].filter(Boolean).join(' on ') || 'Passkey';
}
