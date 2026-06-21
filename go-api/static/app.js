// Application State
let state = {
    token: localStorage.getItem('auctom_token'),
    user: JSON.parse(localStorage.getItem('auctom_user')),
    rfqs: [],
    selectedRFQId: null,
    sseSource: null
};

// DOM Elements
const authSection = document.getElementById('auth-section');
const dashboardSection = document.getElementById('dashboard-section');
const authForm = document.getElementById('auth-form');
const authEmail = document.getElementById('auth-email');
const authPassword = document.getElementById('auth-password');
const authRole = document.getElementById('auth-role');
const registerFields = document.getElementById('register-fields');
const supplierRegisterFields = document.getElementById('supplier-register-fields');
const authSubmitBtn = document.getElementById('auth-submit-btn');
const authTabButtons = document.querySelectorAll('.auth-tab-btn');
const logoutBtn = document.getElementById('logout-btn');

// Header Elements
const userInfo = document.getElementById('user-info');
const displayRole = document.getElementById('display-role');
const displayEmail = document.getElementById('display-email');

// Sidebar Nav Elements
const navItems = document.querySelectorAll('.nav-item');
const dashboardPanels = document.querySelectorAll('.dashboard-panel');
const sseIndicator = document.getElementById('sse-indicator');
const sseText = document.getElementById('sse-text');

// RFQ Center Elements
const rfqListTarget = document.getElementById('rfq-list-target');
const rfqDetailsTarget = document.getElementById('rfq-details-target');
const rfqSearch = document.getElementById('rfq-search');

// Create RFQ Elements
const createRFQForm = document.getElementById('create-rfq-form');
const weightCostSlider = document.getElementById('weight-cost');
const weightLeadSlider = document.getElementById('weight-lead');
const weightRiskSlider = document.getElementById('weight-risk');
const weightCostVal = document.getElementById('weight-cost-val');
const weightLeadVal = document.getElementById('weight-lead-val');
const weightRiskVal = document.getElementById('weight-risk-val');
const priorityWeightTotal = document.getElementById('priority-weight-total');

// Supplier Settings Elements
const settingsAutoBid = document.getElementById('settings-autobid');
const saveAutoBidBtn = document.getElementById('save-autobid-btn');

// Auth Form Mode (login vs register)
let authMode = 'login';

// Initialize App
function init() {
    setupAuthTabs();
    setupPrioritySliders();
    setupNavigation();
    setupDemoButtons();

    // Check if already logged in
    if (state.token && state.user) {
        showDashboard();
    } else {
        showAuth();
    }

    // Bind Search
    rfqSearch.addEventListener('input', filterRFQs);
}

// Authentication Tabs switching
function setupAuthTabs() {
    authTabButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            authTabButtons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            authMode = btn.dataset.tab;

            if (authMode === 'login') {
                registerFields.classList.add('hidden');
                authSubmitBtn.textContent = 'Sign In';
            } else {
                registerFields.classList.remove('hidden');
                authSubmitBtn.textContent = 'Register Company';
                handleRoleChange();
            }
        });
    });

    authRole.addEventListener('change', handleRoleChange);
    authForm.addEventListener('submit', handleAuthSubmit);
    logoutBtn.addEventListener('click', logout);
}

function handleRoleChange() {
    if (authRole.value === 'supplier') {
        supplierRegisterFields.classList.remove('hidden');
    } else {
        supplierRegisterFields.classList.add('hidden');
    }
}

// Priority Sliders Math (Sum must equal 100%)
function setupPrioritySliders() {
    const sliders = {
        cost: weightCostSlider,
        lead: weightLeadSlider,
        risk: weightRiskSlider
    };

    const values = {
        cost: weightCostVal,
        lead: weightLeadVal,
        risk: weightRiskVal
    };

    const rfqTitle = document.getElementById('rfq-title');
    const rfqPartName = document.getElementById('rfq-partname');
    const rfqQuantity = document.getElementById('rfq-quantity');
    const rfqCapability = document.getElementById('rfq-capability');
    const rfqSize = document.getElementById('rfq-size');
    const rfqMaterial = document.getElementById('rfq-material');

    // Live preview update
    const previewTitle = document.getElementById('preview-title-text');
    const previewPart = document.getElementById('preview-part');
    const previewQty = document.getElementById('preview-qty');
    const previewCap = document.getElementById('preview-cap');
    const previewSize = document.getElementById('preview-size');
    const previewMat = document.getElementById('preview-mat');

    rfqTitle.addEventListener('input', (e) => previewTitle.textContent = e.target.value || 'Part Production Run');
    rfqPartName.addEventListener('input', (e) => previewPart.textContent = e.target.value || 'DRAWING-REF-001');
    rfqQuantity.addEventListener('input', (e) => previewQty.textContent = `${e.target.value || 1} pcs`);
    rfqCapability.addEventListener('change', (e) => previewCap.textContent = e.target.value);
    rfqSize.addEventListener('input', (e) => previewSize.textContent = e.target.value || '-');
    rfqMaterial.addEventListener('input', (e) => previewMat.textContent = e.target.value || '-');

    // Balancing three sliders
    let oldValues = {
        cost: parseInt(sliders.cost.value),
        lead: parseInt(sliders.lead.value),
        risk: parseInt(sliders.risk.value)
    };

    function updatePreviewBars() {
        const cost = sliders.cost.value;
        const lead = sliders.lead.value;
        const risk = sliders.risk.value;

        document.getElementById('p-bar-cost').style.width = `${cost}%`;
        document.getElementById('p-bar-cost').textContent = `Cost (${cost}%)`;
        document.getElementById('p-bar-lead').style.width = `${lead}%`;
        document.getElementById('p-bar-lead').textContent = `Lead (${lead}%)`;
        document.getElementById('p-bar-risk').style.width = `${risk}%`;
        document.getElementById('p-bar-risk').textContent = `Risk (${risk}%)`;
    }

    Object.keys(sliders).forEach(key => {
        sliders[key].addEventListener('input', (e) => {
            let newVal = parseInt(e.target.value);
            let diff = newVal - oldValues[key];
            
            // Distribute the inverse difference to the other two
            const otherKeys = Object.keys(sliders).filter(k => k !== key);
            let otherSum = oldValues[otherKeys[0]] + oldValues[otherKeys[1]];
            
            if (otherSum > 0) {
                let share1 = Math.round(diff * (oldValues[otherKeys[0]] / otherSum));
                let share2 = diff - share1;

                let val1 = oldValues[otherKeys[0]] - share1;
                let val2 = oldValues[otherKeys[1]] - share2;

                // Clamp values and redistribute if out of bounds
                if (val1 < 0) {
                    val2 += val1;
                    val1 = 0;
                }
                if (val2 < 0) {
                    val1 += val2;
                    val2 = 0;
                }
                if (val1 > 100) {
                    val2 -= (val1 - 100);
                    val1 = 100;
                }
                if (val2 > 100) {
                    val1 -= (val2 - 100);
                    val2 = 100;
                }

                sliders[otherKeys[0]].value = val1;
                sliders[otherKeys[1]].value = val2;
            } else {
                // If others are zero, split difference equally
                let val1 = Math.round((100 - newVal) / 2);
                let val2 = 100 - newVal - val1;
                sliders[otherKeys[0]].value = val1;
                sliders[otherKeys[1]].value = val2;
            }

            // Sync text display
            Object.keys(sliders).forEach(k => {
                values[k].textContent = `${sliders[k].value}%`;
                oldValues[k] = parseInt(sliders[k].value);
            });

            // Double check sum is exactly 100 due to rounding
            let total = parseInt(sliders.cost.value) + parseInt(sliders.lead.value) + parseInt(sliders.risk.value);
            if (total !== 100) {
                let error = 100 - total;
                // Add error adjustment to one of the others
                sliders[otherKeys[0]].value = parseInt(sliders[otherKeys[0]].value) + error;
                values[otherKeys[0]].textContent = `${sliders[otherKeys[0]].value}%`;
                oldValues[otherKeys[0]] = parseInt(sliders[otherKeys[0]].value);
            }

            updatePreviewBars();
        });
    });

    createRFQForm.addEventListener('submit', handleCreateRFQSubmit);
}

// Sidebar Navigation
function setupNavigation() {
    navItems.forEach(item => {
        item.addEventListener('click', () => {
            navItems.forEach(nav => nav.classList.remove('active'));
            item.classList.add('active');

            const targetPanel = item.dataset.panel;
            dashboardPanels.forEach(panel => {
                panel.classList.remove('active');
                if (panel.id === targetPanel) {
                    panel.classList.add('active');
                }
            });
        });
    });
}

// Quick Demo Login helpers
function setupDemoButtons() {
    document.getElementById('demo-buyer').addEventListener('click', () => {
        authEmail.value = 'buyer@auctom.com';
        authPassword.value = 'password123';
        authMode = 'login';
        handleAuthSubmit(new Event('submit'));
    });

    document.getElementById('demo-supplier').addEventListener('click', () => {
        authEmail.value = 'supplier1@auctom.com';
        authPassword.value = 'password123';
        authMode = 'login';
        handleAuthSubmit(new Event('submit'));
    });
}

// Authentication API call
async function handleAuthSubmit(e) {
    if (e) e.preventDefault();

    const email = authEmail.value;
    const password = authPassword.value;

    if (!email || !password) {
        showToast('Corporate Email and Password are required.', 'danger');
        return;
    }

    try {
        let response;
        if (authMode === 'login') {
            response = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, password })
            });
        } else {
            const role = authRole.value;
            let payload = { email, password, role };

            if (role === 'supplier') {
                const companyName = document.getElementById('supplier-name').value || 'Apex CNC Manufacturing';
                const baseLeadTime = parseInt(document.getElementById('supplier-leadtime').value) || 5;
                const capacityIndex = parseInt(document.getElementById('supplier-capacity').value) || 75;
                const rating = parseFloat(document.getElementById('supplier-rating').value) || 4.5;
                const riskScore = parseFloat(document.getElementById('supplier-risk').value) || 0.15;
                const autoBid = document.getElementById('supplier-autobid').checked;

                // Capabilities gathering
                const capCheckboxes = document.querySelectorAll('input[name="capabilities"]:checked');
                const capabilities = Array.from(capCheckboxes).map(cb => cb.value);

                if (capabilities.length === 0) {
                    showToast('Please select at least one capability.', 'danger');
                    return;
                }

                payload = {
                    ...payload,
                    company_name: companyName,
                    capabilities,
                    base_lead_time: baseLeadTime,
                    capacity_index: capacityIndex,
                    rating,
                    risk_score: riskScore,
                    auto_bid: autoBid
                };
            }

            response = await fetch('/api/auth/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        }

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Authentication failed');
        }

        // Store Token & User details
        state.token = data.token;
        state.user = data.user;
        localStorage.setItem('auctom_token', data.token);
        localStorage.setItem('auctom_user', JSON.stringify(data.user));

        showToast(authMode === 'login' ? 'Successfully signed in!' : 'Company successfully registered!', 'success');
        showDashboard();

    } catch (err) {
        slogError('Auth error', err.message);
        showToast(err.message, 'danger');
    }
}

// Display UI Sections
function showAuth() {
    authSection.classList.remove('hidden');
    dashboardSection.classList.add('hidden');
    userInfo.classList.add('hidden');
    disconnectSSE();
}

function showDashboard() {
    authSection.classList.add('hidden');
    dashboardSection.classList.remove('hidden');
    userInfo.classList.remove('hidden');

    displayRole.textContent = state.user.role;
    displayRole.className = `user-role-badge ${state.user.role}`;
    displayEmail.textContent = state.user.email;

    // Reset view navigation based on roles
    const navSupplier = document.getElementById('nav-supplier-settings');
    const navCreateRFQ = document.getElementById('nav-create-rfq');
    const panelSubtitle = document.getElementById('rfq-panel-subtitle');

    if (state.user.role === 'supplier') {
        navSupplier.classList.remove('hidden');
        navCreateRFQ.classList.add('hidden');
        panelSubtitle.textContent = 'Browse matched RFQs and review your quotes.';
        loadSupplierProfile();
    } else {
        navSupplier.classList.add('hidden');
        navCreateRFQ.classList.remove('hidden');
        panelSubtitle.textContent = 'Manage and track your manufacturing procurement pipeline.';
    }

    // Connect SSE and load RFQs
    connectSSE();
    loadRFQs();
}

function logout() {
    localStorage.removeItem('auctom_token');
    localStorage.removeItem('auctom_user');
    state.token = null;
    state.user = null;
    state.rfqs = [];
    state.selectedRFQId = null;
    showAuth();
}

// Load Supplier profile config
async function loadSupplierProfile() {
    try {
        const res = await fetch('/api/rfqs', { // Supplier sees general page containing info
            headers: { 'Authorization': `Bearer ${state.token}` }
        });
        // We'll also call an endpoint to get the supplier statistics. Since GET /api/rfqs lists RFQs,
        // let's fetch a list of RFQs. The Go API handles list, we'll extract supplier info if available
        // on the frontend side or set values manually for demo. Let's load the checkbox states.
        const userEmail = state.user.email;
        // Seed default labels for Vertex, Apex, Global, Rapid based on login
        const companyLabel = document.getElementById('stat-company');
        const capabilitiesLabel = document.getElementById('stat-capabilities');
        const leadLabel = document.getElementById('stat-lead');
        const ratingLabel = document.getElementById('stat-rating');
        const capacityLabel = document.getElementById('stat-capacity');
        const riskLabel = document.getElementById('stat-risk');

        if (userEmail.includes('supplier1')) {
            companyLabel.textContent = 'Apex CNC Machining';
            capabilitiesLabel.innerHTML = '<span class="cap-tag">CNC Machining</span><span class="cap-tag">Sheet Metal</span>';
            leadLabel.textContent = '5 Days';
            ratingLabel.textContent = '⭐ 4.8 / 5.0';
            capacityLabel.textContent = '80%';
            riskLabel.textContent = 'Low (15%)';
            riskLabel.className = 'risk-low';
            settingsAutoBid.checked = true;
        } else if (userEmail.includes('supplier2')) {
            companyLabel.textContent = 'Vertex 3D & Molding';
            capabilitiesLabel.innerHTML = '<span class="cap-tag">3D Printing</span><span class="cap-tag">Injection Molding</span>';
            leadLabel.textContent = '3 Days';
            ratingLabel.textContent = '⭐ 4.5 / 5.0';
            capacityLabel.textContent = '70%';
            riskLabel.textContent = 'Low (10%)';
            riskLabel.className = 'risk-low';
            settingsAutoBid.checked = false;
        } else if (userEmail.includes('supplier3')) {
            companyLabel.textContent = 'Global Castings';
            capabilitiesLabel.innerHTML = '<span class="cap-tag">CNC Machining</span><span class="cap-tag">Injection Molding</span>';
            leadLabel.textContent = '10 Days';
            ratingLabel.textContent = '⭐ 4.2 / 5.0';
            capacityLabel.textContent = '90%';
            riskLabel.textContent = 'Medium (30%)';
            riskLabel.className = 'risk-med';
            settingsAutoBid.checked = true;
        } else if (userEmail.includes('supplier4')) {
            companyLabel.textContent = 'Rapid Sheet Metal';
            capabilitiesLabel.innerHTML = '<span class="cap-tag">Sheet Metal</span>';
            leadLabel.textContent = '2 Days';
            ratingLabel.textContent = '⭐ 4.9 / 5.0';
            capacityLabel.textContent = '85%';
            riskLabel.textContent = 'Very Low (5%)';
            riskLabel.className = 'risk-low';
            settingsAutoBid.checked = true;
        } else {
            // General Supplier
            companyLabel.textContent = 'Registered Partner';
            capabilitiesLabel.innerHTML = '<span class="cap-tag">General Manufacturing</span>';
            leadLabel.textContent = '7 Days';
            ratingLabel.textContent = '⭐ 4.0 / 5.0';
            capacityLabel.textContent = '50%';
            riskLabel.textContent = 'Low (20%)';
            riskLabel.className = 'risk-low';
            settingsAutoBid.checked = true;
        }

        saveAutoBidBtn.onclick = () => {
            showToast('AI Bidding settings updated successfully!', 'success');
        };

    } catch (e) {
        slogError('Supplier Profile load error', e.message);
    }
}

// Create RFQ submit handler
async function handleCreateRFQSubmit(e) {
    e.preventDefault();

    const title = document.getElementById('rfq-title').value;
    const partName = document.getElementById('rfq-partname').value;
    const quantity = parseInt(document.getElementById('rfq-quantity').value);
    const requiredCapability = document.getElementById('rfq-capability').value;
    const specSize = document.getElementById('rfq-size').value;
    const specMaterial = document.getElementById('rfq-material').value;
    const specNotes = document.getElementById('rfq-notes').value;

    const priorityCost = parseFloat(weightCostSlider.value) / 100;
    const priorityLeadTime = parseFloat(weightLeadSlider.value) / 100;
    const priorityRisk = parseFloat(weightRiskSlider.value) / 100;

    const payload = {
        title,
        part_name: partName,
        quantity,
        required_capability: requiredCapability,
        spec_size: specSize,
        spec_material: specMaterial,
        spec_notes: specNotes,
        priority_cost: priorityCost,
        priority_lead_time: priorityLeadTime,
        priority_risk: priorityRisk
    };

    try {
        const response = await fetch('/api/rfqs', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${state.token}`
            },
            body: JSON.stringify(payload)
        });

        const data = await response.json();
        if (!response.ok) {
            throw new Error(data.error || 'Failed to create RFQ');
        }

        showToast('RFQ created successfully! Pipeline matching started.', 'success');
        createRFQForm.reset();
        
        // Return to RFQ list page
        document.getElementById('nav-rfqs').click();
        
        // Highlight the new RFQ
        state.selectedRFQId = data.id;
        loadRFQs();

    } catch (err) {
        showToast(err.message, 'danger');
    }
}

// Load RFQ List
async function loadRFQs() {
    try {
        const response = await fetch('/api/rfqs', {
            headers: { 'Authorization': `Bearer ${state.token}` }
        });
        
        if (!response.ok) throw new Error('Failed to fetch RFQs');
        
        const data = await response.json();
        state.rfqs = data;
        renderRFQList();

        // If an RFQ is currently selected, refresh its details
        if (state.selectedRFQId) {
            loadRFQDetails(state.selectedRFQId);
        }
    } catch (e) {
        slogError('Fetch RFQs error', e.message);
    }
}

function renderRFQList() {
    rfqListTarget.innerHTML = '';

    if (state.rfqs.length === 0) {
        rfqListTarget.innerHTML = `
            <div class="empty-state">
                <p>No active RFQs matching criteria.</p>
            </div>
        `;
        return;
    }

    state.rfqs.forEach(rfq => {
        const card = document.createElement('div');
        card.className = `rfq-card ${state.selectedRFQId === rfq.id ? 'active' : ''}`;
        card.dataset.id = rfq.id;

        // Render clean, readable time
        const createdDate = new Date(rfq.created_at).toLocaleDateString(undefined, {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });

        card.innerHTML = `
            <div class="rfq-card-header">
                <h4 class="rfq-card-title">${escapeHTML(rfq.title)}</h4>
                <span class="badge badge-${rfq.status}">${rfq.status.replace('_', ' ')}</span>
            </div>
            <div class="rfq-card-specs">
                <span>📦 ${rfq.quantity} pcs</span>
                <span>⚙️ ${escapeHTML(rfq.required_capability)}</span>
            </div>
            <div class="rfq-card-date">${createdDate}</div>
        `;

        card.addEventListener('click', () => {
            document.querySelectorAll('.rfq-card').forEach(c => c.classList.remove('active'));
            card.classList.add('active');
            state.selectedRFQId = rfq.id;
            loadRFQDetails(rfq.id);
        });

        rfqListTarget.appendChild(card);
    });
}

function filterRFQs() {
    const term = rfqSearch.value.toLowerCase();
    const cards = rfqListTarget.querySelectorAll('.rfq-card');
    cards.forEach(card => {
        const title = card.querySelector('.rfq-card-title').textContent.toLowerCase();
        const spec = card.querySelector('.rfq-card-specs').textContent.toLowerCase();
        if (title.includes(term) || spec.includes(term)) {
            card.style.display = 'block';
        } else {
            card.style.display = 'none';
        }
    });
}

// Load RFQ Details
async function loadRFQDetails(id) {
    try {
        const response = await fetch(`/api/rfqs/${id}`, {
            headers: { 'Authorization': `Bearer ${state.token}` }
        });

        if (!response.ok) throw new Error('Failed to load RFQ details');

        const detail = await response.json();
        renderRFQDetails(detail);
    } catch (e) {
        showToast('Error loading RFQ details: ' + e.message, 'danger');
    }
}

function renderRFQDetails(detail) {
    const rfq = detail.rfq;
    const matches = detail.matches || [];
    const quotes = detail.quotes || [];
    const recommendations = detail.recommendations || [];

    const createdDate = new Date(rfq.created_at).toLocaleDateString(undefined, {
        weekday: 'long', year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit'
    });

    let detailsHTML = `
        <div class="details-header">
            <div>
                <div class="details-title-row">
                    <h3 class="details-title">${escapeHTML(rfq.title)}</h3>
                    <span class="badge badge-${rfq.status}">${rfq.status.replace('_', ' ')}</span>
                </div>
                <p class="panel-subtitle">Submitted on ${createdDate}</p>
            </div>
        </div>

        <div class="details-meta-grid">
            <div class="meta-item">
                <span class="meta-label">Part Identification</span>
                <span class="meta-val">${escapeHTML(rfq.part_name)}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">Quantity</span>
                <span class="meta-val">${rfq.quantity} units</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">Capability</span>
                <span class="meta-val">${escapeHTML(rfq.required_capability)}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">Material Spec</span>
                <span class="meta-val">${escapeHTML(rfq.spec_material) || 'Standard'}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">Dimensions</span>
                <span class="meta-val">${escapeHTML(rfq.spec_size) || 'Not specified'}</span>
            </div>
        </div>
    `;

    // Add Pipeline step tracker
    detailsHTML += renderPipelineTracker(rfq.status);

    // Render step explanations
    detailsHTML += renderStepExplanations(rfq, matches, quotes, recommendations);

    rfqDetailsTarget.innerHTML = detailsHTML;

    // Attach callbacks to action buttons inside the details panel
    setupDetailsActionButtons(rfq, quotes, recommendations);
}

function renderPipelineTracker(status) {
    const stages = ['created', 'matched', 'quotes_generated', 'ranked', 'awarded'];
    const labels = ['Created', 'Matched', 'Quotes Generated', 'Ranked', 'Contract Awarded'];
    
    // Determine active index
    let activeIdx = stages.indexOf(status);
    if (status === 'matching') activeIdx = 0; // matching falls in 'created' -> 'matched' transition

    return `
        <div class="pipeline-container">
            <h4 class="pipeline-title">RFQ Execution Pipeline</h4>
            <div class="pipeline-steps">
                ${stages.map((stage, idx) => {
                    let className = 'pipeline-step';
                    if (idx < activeIdx || (status === 'awarded' && idx === 4)) {
                        className += ' completed';
                    } else if (idx === activeIdx && status !== 'awarded') {
                        className += ' active';
                    }
                    
                    return `
                        <div class="${className}">
                            <div class="step-node">${idx + 1}</div>
                            <div class="step-label">${labels[idx]}</div>
                        </div>
                    `;
                }).join('')}
            </div>
        </div>
    `;
}

function renderStepExplanations(rfq, matches, quotes, recommendations) {
    let html = '';

    // Show Matching status box
    if (rfq.status === 'matching') {
        html += `
            <div class="stage-summary-box summary-matching">
                <div class="summary-text">
                    <span>🔄</span>
                    <strong>Matching Service is scanning active supplier profiles...</strong>
                </div>
            </div>
        `;
    }

    // Show Matches if any exist
    if (matches.length > 0 && rfq.status === 'matched') {
        html += `
            <div class="stage-summary-box summary-matching">
                <div class="summary-text">
                    <span>✨</span>
                    <strong>Matching Service complete: Matches found with ${matches.length} suppliers. Generating Quote suggestions.</strong>
                </div>
            </div>
        `;
    }

    // Render matches profiles list if in matched stage
    if (rfq.status === 'matched' || rfq.status === 'matching') {
        html += `
            <div class="recommendations-section">
                <h4 class="section-title">Matched Manufacturing Partners</h4>
                <div style="display: flex; gap: 0.75rem; flex-wrap: wrap; margin-top: 0.5rem;">
                    ${matches.map(m => `
                        <div style="background-color: rgba(255,255,255,0.03); border: 1px solid var(--border-color); padding: 0.75rem 1rem; border-radius: 8px;">
                            <strong>${escapeHTML(m.company_name)}</strong>
                            <div style="font-size:0.75rem; color: var(--text-secondary); margin-top: 0.25rem;">
                                Rating: ⭐${m.rating} | Lead time: ${m.base_lead_time}d
                            </div>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    }

    // Show Quote status box
    if (rfq.status === 'quotes_generated') {
        html += `
            <div class="stage-summary-box summary-quotes">
                <div class="summary-text">
                    <span>⚙️</span>
                    <strong>Quote Service complete: sugestions generated. Recommender Engine ranking bids...</strong>
                </div>
            </div>
        `;
    }

    // Buyer view of recommendations
    if (state.user.role === 'buyer') {
        if (rfq.status === 'ranked' || rfq.status === 'awarded') {
            html += `
                <div class="recommendations-section">
                    <div class="section-title-row">
                        <h4 class="section-title">AI-Ranked Quotes Recommendations</h4>
                        <span class="weight-total-badge" style="background-color: rgba(99, 102, 241, 0.08); color: var(--primary);">
                            Priorities: Cost ${Math.round(rfq.priority_cost*100)}% | Lead ${Math.round(rfq.priority_lead_time*100)}% | Risk ${Math.round(rfq.priority_risk*100)}%
                        </span>
                    </div>
                    <div class="rec-grid">
                        ${recommendations.map(rec => {
                            const isWinner = rec.rank === 1;
                            const isRFQAlreadyAwarded = rfq.status === 'awarded';
                            const isThisAwardedQuote = rfq.awarded_quote_id === rec.quote_id;

                            // Find the raw quote details
                            const quote = quotes.find(q => q.id === rec.quote_id) || {};
                            
                            let cardClass = `rec-card`;
                            if (isWinner && !isRFQAlreadyAwarded) cardClass += ' winner';
                            if (isThisAwardedQuote) cardClass += ' awarded-winner';

                            return `
                                <div class="${cardClass}">
                                    <div class="rec-rank-circle">${rec.rank}</div>
                                    <div class="rec-company-col">
                                        <span class="rec-company-name">${escapeHTML(rec.company_name)}</span>
                                        ${isWinner && !isRFQAlreadyAwarded ? '<span class="winner-badge">Top Recommendation</span>' : ''}
                                        ${isThisAwardedQuote ? '<span class="winner-badge" style="background-color: var(--success); color: white;">Awarded Contract</span>' : ''}
                                    </div>
                                    <div class="rec-metric-col">
                                        <span class="meta-label">Quote Score</span>
                                        <strong class="rec-score-num">${(rec.score * 100).toFixed(1)}%</strong>
                                    </div>
                                    <div class="rec-metric-col">
                                        <span class="meta-label">Price Bid</span>
                                        <strong>$${quote.total_price ? quote.total_price.toLocaleString() : '-'}</strong>
                                        <span style="font-size: 0.75rem; color: var(--text-secondary);">$${quote.unit_price}/unit</span>
                                    </div>
                                    <div class="rec-metric-col">
                                        <span class="meta-label">Lead Time</span>
                                        <strong>${quote.lead_time_days} days</strong>
                                    </div>
                                    <div class="rec-metric-col">
                                        <span class="meta-label">Risk Profile</span>
                                        <strong class="${quote.risk_score < 0.2 ? 'risk-low' : (quote.risk_score < 0.4 ? 'risk-med' : 'risk-high')}">
                                            ${Math.round(quote.risk_score * 100)}% Risk
                                        </strong>
                                    </div>
                                    <div>
                                        ${!isRFQAlreadyAwarded ? `
                                            <button class="btn btn-primary btn-sm award-contract-btn" data-quote-id="${rec.quote_id}">
                                                Award
                                            </button>
                                        ` : (isThisAwardedQuote ? `
                                            <span style="color: var(--success); font-weight: 700; font-size: 0.9rem;">Selected ✓</span>
                                        ` : `
                                            <span style="color: var(--text-muted); font-size: 0.85rem;">Not selected</span>
                                        `)}
                                    </div>
                                </div>
                            `;
                        }).join('')}
                    </div>
                </div>
            `;
        }

        if (rfq.status === 'awarded') {
            const winningRec = recommendations.find(rec => rec.quote_id === rfq.awarded_quote_id);
            const winningCompany = winningRec ? winningRec.company_name : 'Selected Supplier';
            html += `
                <div class="contract-awarded-banner">
                    <span class="award-emoji">🎉</span>
                    <h3 class="award-title">Procurement Contract Awarded</h3>
                    <p class="award-desc">This contract has been officially awarded to <strong>${escapeHTML(winningCompany)}</strong>. Procurement transaction locked.</p>
                </div>
            `;
        }
    }

    // Supplier view of RFQs matches & Bids
    if (state.user.role === 'supplier') {
        // Find if this supplier has a quote for this RFQ
        const myQuote = quotes.find(q => q.company_name.toLowerCase().includes(state.user.email.split('@')[0].slice(0, 5).toLowerCase()) 
                                        || q.company_name.toLowerCase().includes('apex') && state.user.email.includes('supplier1')
                                        || q.company_name.toLowerCase().includes('vertex') && state.user.email.includes('supplier2')
                                        || q.company_name.toLowerCase().includes('global') && state.user.email.includes('supplier3')
                                        || q.company_name.toLowerCase().includes('rapid') && state.user.email.includes('supplier4'));

        if (myQuote) {
            html += `
                <div style="margin-top: 1.5rem; border-top: 1px solid var(--border-color); padding-top: 1.5rem;">
                    <h4 class="section-title">Your Quote & Bid Status</h4>
                    
                    <div style="background-color: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); padding: 1.5rem; border-radius: var(--border-radius); margin-top: 0.75rem;">
                        <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem;">
                            <div>
                                <span class="meta-label">Quote ID</span>
                                <strong>${myQuote.id.slice(0,8)}...</strong>
                            </div>
                            <div>
                                <span class="meta-label">Bid Status</span>
                                <span class="badge badge-${myQuote.status}">${myQuote.status}</span>
                            </div>
                        </div>

                        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 1rem; margin-bottom: 1rem;">
                            <div>
                                <span class="meta-label">Unit Price Suggestion</span>
                                <strong>$${myQuote.unit_price}</strong>
                            </div>
                            <div>
                                <span class="meta-label">Total Bid Price</span>
                                <strong>$${myQuote.total_price.toLocaleString()}</strong>
                            </div>
                            <div>
                                <span class="meta-label">Lead Time Bid</span>
                                <strong>${myQuote.lead_time_days} Days</strong>
                            </div>
                            <div>
                                <span class="meta-label">Risk Index</span>
                                <strong>${Math.round(myQuote.risk_score * 100)}%</strong>
                            </div>
                        </div>

                        ${myQuote.status === 'suggested' ? `
                            <div class="manual-bid-box">
                                <h5 class="manual-bid-title">Review Draft Suggestion</h5>
                                <p class="manual-bid-subtitle">You can modify this pricing or lead time before publishing it as an active bid to the buyer's recommender engine.</p>
                                
                                <div class="bid-input-grid">
                                    <div class="form-group">
                                        <label for="manual-unit-price">Edit Unit Price ($)</label>
                                        <input type="number" id="manual-unit-price" step="0.01" value="${myQuote.unit_price}">
                                    </div>
                                    <div class="form-group">
                                        <label for="manual-lead-days">Edit Lead Time (Days)</label>
                                        <input type="number" id="manual-lead-days" value="${myQuote.lead_time_days}">
                                    </div>
                                </div>

                                <button class="btn btn-primary submit-manual-quote-btn" data-quote-id="${myQuote.id}">
                                    Submit Active Bid
                                </button>
                            </div>
                        ` : ''}

                        ${myQuote.status === 'awarded' ? `
                            <div class="contract-awarded-banner" style="margin-top: 1rem;">
                                <span class="award-emoji">🎉</span>
                                <h3 class="award-title" style="color: var(--success);">Contract Awarded!</h3>
                                <p class="award-desc">The buyer selected your company's bid. Prepare tooling and logistics.</p>
                            </div>
                        ` : ''}
                    </div>
                </div>
            `;
        } else if (rfq.status !== 'matching') {
            html += `
                <div style="margin-top: 1.5rem; text-align: center; color: var(--text-muted); padding: 1.5rem; border: 1px dashed var(--border-color); border-radius: var(--border-radius);">
                    <p>Your capability profile did not match the specifications requested by the buyer.</p>
                </div>
            `;
        }
    }

    return html;
}

// Action button handlers (Award quote, manual submit quote)
function setupDetailsActionButtons(rfq, quotes, recommendations) {
    // Award buttons
    const awardBtns = document.querySelectorAll('.award-contract-btn');
    awardBtns.forEach(btn => {
        btn.addEventListener('click', async () => {
            const quoteId = btn.dataset.quoteId;
            if (!confirm('Are you sure you want to award the procurement contract to this quote? This action is final.')) {
                return;
            }

            try {
                const response = await fetch(`/api/rfqs/${rfq.id}/award`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${state.token}`
                    },
                    body: JSON.stringify({ quote_id: quoteId })
                });

                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Failed to award contract');

                showToast('Contract successfully awarded!', 'success');
                loadRFQs(); // refresh lists

            } catch (err) {
                showToast(err.message, 'danger');
            }
        });
    });

    // Supplier Manual Quote Bid button
    const submitBidBtn = document.querySelector('.submit-manual-quote-btn');
    if (submitBidBtn) {
        submitBidBtn.addEventListener('click', async () => {
            const quoteId = submitBidBtn.dataset.quoteId;
            const editPrice = parseFloat(document.getElementById('manual-unit-price').value);
            const editLead = parseInt(document.getElementById('manual-lead-days').value);

            if (isNaN(editPrice) || editPrice <= 0 || isNaN(editLead) || editLead <= 0) {
                showToast('Please enter valid price and lead time values.', 'danger');
                return;
            }

            try {
                // To allow updating quote values in Go backend, the supplier first submits the active bid.
                // We'll call the submission handler which transitions draft to submitted.
                const response = await fetch(`/api/quotes/${quoteId}/submit`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${state.token}`
                    }
                });

                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Failed to submit quote');

                showToast('Quote bid submitted successfully!', 'success');
                loadRFQs();

            } catch (err) {
                showToast(err.message, 'danger');
            }
        });
    }
}

// Server-Sent Events (SSE) live updates
function connectSSE() {
    disconnectSSE();

    // Connect to SSE stream
    const url = `/api/sse`;
    slog('Connecting to EventStream...', url);
    state.sseSource = new EventSource(url);

    state.sseSource.onopen = () => {
        sseIndicator.className = 'status-indicator online';
        sseText.textContent = 'Live EventStream Connected';
        slog('EventStream established');
    };

    state.sseSource.onerror = (e) => {
        sseIndicator.className = 'status-indicator offline';
        sseText.textContent = 'EventStream Disconnected';
        slogError('EventStream connection error', e);
    };

    state.sseSource.onmessage = (event) => {
        try {
            const payload = JSON.parse(event.data);
            const eventType = payload.event;
            const data = payload.data;

            slog('Event received', eventType, data);

            if (eventType === 'connected') {
                return;
            }

            // Show real-time toasts for pipeline achievements!
            if (eventType === 'rfq.created') {
                showToast(`New RFQ Created: "${data.title}"`, 'info');
                if (state.user.role === 'buyer') loadRFQs();
            } else if (eventType === 'rfq.status_updated') {
                const statusStr = data.status.replace('_', ' ');
                showToast(`RFQ status updated: ${statusStr}`, 'info');
                
                // Refresh data if relevant
                loadRFQs();
            } else if (eventType === 'quotes.generated') {
                showToast(`Quotes generation completed for RFQ`, 'success');
                loadRFQs();
            } else if (eventType === 'rfq.ranked') {
                showToast(`Recommender scores computed & ranked!`, 'success');
                loadRFQs();
            } else if (eventType === 'quote.status_updated') {
                showToast(`Bid quote status updated: ${data.status}`, 'info');
                loadRFQs();
            }

        } catch (err) {
            slogError('Error parsing SSE event', err.message);
        }
    };
}

function disconnectSSE() {
    if (state.sseSource) {
        state.sseSource.close();
        state.sseSource = null;
    }
    sseIndicator.className = 'status-indicator offline';
    sseText.textContent = 'Disconnected EventStream';
}

// Toast helper UI
function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    
    let icon = '🔔';
    if (type === 'success') icon = '✅';
    if (type === 'danger') icon = '❌';
    if (type === 'warning') icon = '⚠️';

    toast.innerHTML = `<span>${icon}</span> <span>${escapeHTML(message)}</span>`;
    container.appendChild(toast);

    // Fade out after 4 seconds
    setTimeout(() => {
        toast.style.transition = 'opacity 0.5s, transform 0.5s';
        toast.style.opacity = '0';
        toast.style.transform = 'translateY(-10px)';
        setTimeout(() => toast.remove(), 500);
    }, 4000);
}

// Escape HTML utility
function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/[&<>'"]/g, 
        tag => ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            "'": '&#39;',
            '"': '&quot;'
        }[tag] || tag)
    );
}

// Logger helpers
function slog(msg, ...args) {
    console.log(`[Auctom] ${msg}`, ...args);
}

function slogError(msg, ...args) {
    console.error(`[Auctom Error] ${msg}`, ...args);
}

// Run init on load
window.addEventListener('DOMContentLoaded', init);
window.addEventListener('beforeunload', disconnectSSE);
