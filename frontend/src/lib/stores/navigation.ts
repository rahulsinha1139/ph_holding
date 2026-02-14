import { writable } from 'svelte/store';

// Pending DN creation - set by OrdersScreen, consumed by DeliveryNotesScreen
interface PendingDNCreate {
    orderId: string;
    orderNumber: string;
    customerName: string;
    timestamp: number;
}

function createPendingDNStore() {
    const { subscribe, set } = writable<PendingDNCreate | null>(null);

    return {
        subscribe,
        request: (orderId: string, orderNumber: string, customerName: string) => {
            set({ orderId, orderNumber, customerName, timestamp: Date.now() });
        },
        clear: () => set(null)
    };
}

export const pendingDNCreate = createPendingDNStore();
