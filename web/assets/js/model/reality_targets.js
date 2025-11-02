// List of popular services for VLESS Reality Target/SNI randomization
const REALITY_TARGETS = [
    { target: 'www.icloud.com:443', sni: 'www.icloud.com,icloud.com' },
    { target: 'www.apple.com:443', sni: 'www.apple.com,apple.com' },
    { target: 'www.tesla.com:443', sni: 'www.tesla.com,tesla.com' },
    { target: 'www.sony.com:443', sni: 'www.sony.com,sony.com' },
    { target: 'www.nvidia.com:443', sni: 'www.nvidia.com,nvidia.com' },
    { target: 'www.amd.com:443', sni: 'www.amd.com,amd.com' },
    { target: 'azure.microsoft.com:443', sni: 'azure.microsoft.com,www.azure.com' },
    { target: 'aws.amazon.com:443', sni: 'aws.amazon.com,amazon.com' },
    { target: 'www.bing.com:443', sni: 'www.bing.com,bing.com' },
    { target: 'www.oracle.com:443', sni: 'www.oracle.com,oracle.com' },
    { target: 'www.intel.com:443', sni: 'www.intel.com,intel.com' },
    { target: 'www.microsoft.com:443', sni: 'www.microsoft.com,microsoft.com' },
    { target: 'www.amazon.com:443', sni: 'www.amazon.com,amazon.com' }
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

