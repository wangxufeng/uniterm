export interface ScanResult {
    keys: RedisKeyInfo[]
    cursor: number
    scanCount: number
}

export interface RedisKeyInfo {
    name: string
    type: 'string' | 'hash' | 'list' | 'set' | 'zset'
    ttl: number
}

export interface FieldEntry {
    field: string
    value: string
}

export interface ScoredMember {
    score: number
    member: string
}
