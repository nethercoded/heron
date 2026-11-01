type Fetch = typeof fetch;

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly fields: Record<string, string>;

	constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
		super(message);
		this.status = status;
		this.code = code;
		this.fields = fields;
	}

	field(name: string) {
		return this.fields[name];
	}
}

async function request<T>(method: string, path: string, body?: unknown, fetcher: Fetch = fetch): Promise<T> {
	const init: RequestInit = { method, headers: { Accept: 'application/json' }, credentials: 'same-origin' };
	if (body !== undefined) {
		init.headers = { ...init.headers, 'Content-Type': 'application/json' };
		init.body = JSON.stringify(body);
	}

	let res: Response;
	try {
		res = await fetcher(`/api${path}`, init);
	} catch {
		throw new ApiError(0, 'network', 'Could not reach the server');
	}

	if (res.status === 204 || res.status === 202) {
		return undefined as T;
	}

	const data = await res.json().catch(() => null);
	if (!res.ok) {
		throw new ApiError(res.status, data?.code ?? 'unknown', data?.message ?? res.statusText, data?.fields ?? {});
	}
	return data as T;
}

export const api = {
	get: <T>(path: string, fetcher?: Fetch) => request<T>('GET', path, undefined, fetcher),
	post: <T = void>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
	put: <T = void>(path: string, body: unknown) => request<T>('PUT', path, body),
	delete: (path: string) => request<void>('DELETE', path)
};

export function message(err: unknown) {
	if (err instanceof ApiError) {
		const fields = Object.entries(err.fields);
		if (fields.length > 0 && err.code === 'invalid') {
			return 'Please fix the highlighted fields';
		}
		return err.message;
	}
	return err instanceof Error ? err.message : 'Something went wrong';
}
