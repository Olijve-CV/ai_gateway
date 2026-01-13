// AI Gateway - Main JavaScript

// Token management
const TokenManager = {
    get() {
        return localStorage.getItem('token');
    },

    set(token) {
        localStorage.setItem('token', token);
    },

    remove() {
        localStorage.removeItem('token');
    },

    isLoggedIn() {
        return !!this.get();
    }
};

// API client
const api = {
    async request(url, options = {}) {
        const token = TokenManager.get();
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };

        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const response = await fetch(url, {
            ...options,
            headers
        });

        if (response.status === 401) {
            TokenManager.remove();
            window.location.href = '/login';
            return null;
        }

        return response;
    },

    async get(url) {
        return this.request(url);
    },

    async post(url, data) {
        return this.request(url, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    },

    async put(url, data) {
        return this.request(url, {
            method: 'PUT',
            body: data ? JSON.stringify(data) : undefined
        });
    },

    async delete(url) {
        return this.request(url, {
            method: 'DELETE'
        });
    }
};

// Utility functions
function showAlert(message, type = 'success') {
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type}`;
    alertDiv.textContent = message;
    alertDiv.style.position = 'fixed';
    alertDiv.style.top = '1rem';
    alertDiv.style.right = '1rem';
    alertDiv.style.zIndex = '9999';
    alertDiv.style.minWidth = '200px';

    document.body.appendChild(alertDiv);

    setTimeout(() => {
        alertDiv.remove();
    }, 3000);
}

// Check authentication on protected pages
function requireAuth() {
    if (!TokenManager.isLoggedIn()) {
        window.location.href = '/login';
        return false;
    }
    return true;
}

// Logout handler
function logout() {
    TokenManager.remove();
    window.location.href = '/login';
}
