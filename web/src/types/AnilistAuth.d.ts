export interface AnilistAuth {
    clientID: string;
    clientSecret: string;
    redirectURL: string;
}

export interface StartAnilistAuthResponse {
    url: string;
}
