export type HealthResponse = {
    status: 'ok'
}

export async function getHealth(signal: AbortSignal): Promise<HealthResponse> {
    const response = await fetch('/api/v1/health', {
        signal: AbortSignal.any([signal, AbortSignal.timeout(5000)]),
    })

    if (!response.ok) {
        throw new Error(`Health request failed: ${response.status}`)
    }

    const data: unknown = await response.json()

    if (
        typeof data !== 'object' ||
        data === null ||
        !('status' in data) ||
        data.status !== 'ok'
    ) {
        throw new Error('Unexpected health response')
    }

    return { status: 'ok' }
}
