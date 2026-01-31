// List of popular services for VLESS Reality Target/SNI randomization
const REALITY_TARGETS = [
    { target: 'www.apple.com:443', sni: 'www.apple.com' },
    { target: 'www.icloud.com:443', sni: 'www.icloud.com' },
    { target: 'www.amazon.com:443', sni: 'www.amazon.com' },
    { target: 'aws.amazon.com:443', sni: 'aws.amazon.com' },
    { target: 'www.oracle.com:443', sni: 'www.oracle.com' },
    { target: 'www.nvidia.com:443', sni: 'www.nvidia.com' },
    { target: 'www.amd.com:443', sni: 'www.amd.com' },
    { target: 'www.intel.com:443', sni: 'www.intel.com' },
    { target: 'www.tesla.com:443', sni: 'www.tesla.com' },
    { target: 'www.sony.com:443', sni: 'www.sony.com' }
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
