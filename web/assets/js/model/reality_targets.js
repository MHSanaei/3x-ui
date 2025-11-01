// List of popular services for VLESS Reality Target/SNI randomization
const REALITY_TARGETS = [
    // CDN & Cloud Infrastructure
    { target: 'www.cloudflare.com:443', sni: 'www.cloudflare.com,cloudflare.com' },
    { target: 'www.microsoft.com:443', sni: 'www.microsoft.com,microsoft.com' },
    { target: 'www.apple.com:443', sni: 'www.apple.com,apple.com' },
    { target: 'www.amazon.com:443', sni: 'www.amazon.com,amazon.com' },
    { target: 'cloud.google.com:443', sni: 'cloud.google.com,www.google.com' },
    { target: 'azure.microsoft.com:443', sni: 'azure.microsoft.com,www.azure.com' },
    { target: 'aws.amazon.com:443', sni: 'aws.amazon.com,amazon.com' },
    { target: 'www.digitalocean.com:443', sni: 'www.digitalocean.com,digitalocean.com' },
    
    // Social Media
    { target: 'www.facebook.com:443', sni: 'www.facebook.com,facebook.com' },
    { target: 'www.instagram.com:443', sni: 'www.instagram.com,instagram.com' },
    { target: 'www.twitter.com:443', sni: 'www.twitter.com,twitter.com' },
    { target: 'www.linkedin.com:443', sni: 'www.linkedin.com,linkedin.com' },
    { target: 'www.reddit.com:443', sni: 'www.reddit.com,reddit.com' },
    { target: 'www.pinterest.com:443', sni: 'www.pinterest.com,pinterest.com' },
    { target: 'www.tumblr.com:443', sni: 'www.tumblr.com,tumblr.com' },
    
    // Video & Streaming
    { target: 'www.youtube.com:443', sni: 'www.youtube.com,youtube.com' },
    { target: 'www.netflix.com:443', sni: 'www.netflix.com,netflix.com' },
    { target: 'www.twitch.tv:443', sni: 'www.twitch.tv,twitch.tv' },
    { target: 'vimeo.com:443', sni: 'vimeo.com,www.vimeo.com' },
    { target: 'www.hulu.com:443', sni: 'www.hulu.com,hulu.com' },
    { target: 'www.disneyplus.com:443', sni: 'www.disneyplus.com,disneyplus.com' },
    
    // News & Media
    { target: 'www.bbc.com:443', sni: 'www.bbc.com,bbc.com' },
    { target: 'www.cnn.com:443', sni: 'www.cnn.com,cnn.com' },
    { target: 'www.nytimes.com:443', sni: 'www.nytimes.com,nytimes.com' },
    { target: 'www.theguardian.com:443', sni: 'www.theguardian.com,theguardian.com' },
    { target: 'www.reuters.com:443', sni: 'www.reuters.com,reuters.com' },
    { target: 'www.bloomberg.com:443', sni: 'www.bloomberg.com,bloomberg.com' },
    
    // E-commerce
    { target: 'www.ebay.com:443', sni: 'www.ebay.com,ebay.com' },
    { target: 'www.alibaba.com:443', sni: 'www.alibaba.com,alibaba.com' },
    { target: 'www.shopify.com:443', sni: 'www.shopify.com,shopify.com' },
    { target: 'www.walmart.com:443', sni: 'www.walmart.com,walmart.com' },
    { target: 'www.target.com:443', sni: 'www.target.com,target.com' },
    
    // Tech Companies
    { target: 'www.github.com:443', sni: 'www.github.com,github.com' },
    { target: 'www.stackoverflow.com:443', sni: 'www.stackoverflow.com,stackoverflow.com' },
    { target: 'www.gitlab.com:443', sni: 'www.gitlab.com,gitlab.com' },
    { target: 'www.docker.com:443', sni: 'www.docker.com,docker.com' },
    { target: 'www.nvidia.com:443', sni: 'www.nvidia.com,nvidia.com' },
    { target: 'www.intel.com:443', sni: 'www.intel.com,intel.com' },
    { target: 'www.amd.com:443', sni: 'www.amd.com,amd.com' },
    
    // Communication & Productivity
    { target: 'www.zoom.us:443', sni: 'www.zoom.us,zoom.us' },
    { target: 'slack.com:443', sni: 'slack.com,www.slack.com' },
    { target: 'www.dropbox.com:443', sni: 'www.dropbox.com,dropbox.com' },
    { target: 'www.notion.so:443', sni: 'www.notion.so,notion.so' },
    { target: 'www.atlassian.com:443', sni: 'www.atlassian.com,atlassian.com' },
    { target: 'www.salesforce.com:443', sni: 'www.salesforce.com,salesforce.com' },
    
    // Search & General
    { target: 'www.wikipedia.org:443', sni: 'www.wikipedia.org,wikipedia.org' },
    { target: 'www.bing.com:443', sni: 'www.bing.com,bing.com' },
    { target: 'www.yahoo.com:443', sni: 'www.yahoo.com,yahoo.com' },
    { target: 'www.duckduckgo.com:443', sni: 'www.duckduckgo.com,duckduckgo.com' },
    
    // Gaming
    { target: 'store.steampowered.com:443', sni: 'store.steampowered.com,steampowered.com' },
    { target: 'www.ea.com:443', sni: 'www.ea.com,ea.com' },
    { target: 'www.epicgames.com:443', sni: 'www.epicgames.com,epicgames.com' },
];

/**
 * Returns a random Reality target configuration from the predefined list
 * @returns {Object} Object with target and sni properties
 */
function getRandomRealityTarget() {
    const randomIndex = Math.floor(Math.random() * REALITY_TARGETS.length);
    const selected = REALITY_TARGETS[randomIndex];
    // Return a copy to avoid reference issues
    return {
        target: selected.target,
        sni: selected.sni
    };
}
