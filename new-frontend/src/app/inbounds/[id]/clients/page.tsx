"use client";

import React, { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useAuth } from '@/context/AuthContext';
import { post } from '@/services/api';
import { Inbound, ClientSetting, Protocol } from '@/types/inbound';
import { formatBytes } from '@/lib/formatters';
import ClientFormModal from '@/components/inbounds/ClientFormModal';
import ClientShareModal from '@/components/inbounds/ClientShareModal'; // Import Share Modal

// Define button styles locally for consistency
const btnPrimaryStyles = "px-4 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors text-sm";
const btnTextPrimaryStyles = "text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 disabled:opacity-50";
const btnTextDangerStyles = "text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50";
const btnTextWarningStyles = "text-yellow-500 hover:text-yellow-700 dark:text-yellow-400 dark:hover:text-yellow-300 disabled:opacity-50";
const btnTextIndigoStyles = "text-indigo-500 hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300 disabled:opacity-50";


interface DisplayClient extends ClientSetting {
  up?: number; down?: number; actualTotal?: number;
  actualExpiryTime?: number; enableClientStat?: boolean;
  inboundId?: number; clientTrafficId?: number;
  originalIndex?: number;
}

enum ClientAction { NONE = '', DELETING = 'deleting', RESETTING_TRAFFIC = 'resetting_traffic' }

const ManageClientsPage: React.FC = () => {
  const params = useParams();
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const inboundId = parseInt(params.id as string, 10);

  const [inbound, setInbound] = useState<Inbound | null>(null);
  const [displayClients, setDisplayClients] = useState<DisplayClient[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [pageError, setPageError] = useState<string | null>(null);

  const [isClientFormModalOpen, setIsClientFormModalOpen] = useState(false);
  const [editingClient, setEditingClient] = useState<DisplayClient | null>(null);
  const [clientFormModalError, setClientFormModalError] = useState<string | null>(null);
  const [clientFormModalLoading, setClientFormModalLoading] = useState(false);

  const [isShareModalOpen, setIsShareModalOpen] = useState(false);
  const [sharingClient, setSharingClient] = useState<ClientSetting | null>(null);

  const [currentAction, setCurrentAction] = useState<ClientAction>(ClientAction.NONE);
  const [actionTargetEmail, setActionTargetEmail] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const fetchInboundAndClients = useCallback(async () => {
    if (!isAuthenticated || !inboundId) return;
    setIsLoading(true); setPageError(null); setActionError(null);
    try {
      const response = await post<Inbound[]>('/inbound/list', {});
      if (response.success && response.data) {
        const currentInbound = response.data.find(ib => ib.id === inboundId);
        if (currentInbound) {
          setInbound(currentInbound);
          let definedClients: ClientSetting[] = [];
          if (currentInbound.protocol === 'vmess' || currentInbound.protocol === 'vless' || currentInbound.protocol === 'trojan') {
            if (currentInbound.settings) {
              try {
                const parsedSettings = JSON.parse(currentInbound.settings);
                if (Array.isArray(parsedSettings.clients)) definedClients = parsedSettings.clients;
              } catch (e) { console.error("Error parsing settings:", e); setPageError("Could not parse client definitions."); }
            }
          }
          const mergedClients: DisplayClient[] = definedClients.map((dc, index) => {
            const stat = currentInbound.clientStats?.find(cs => cs.email === dc.email);
            return {
                ...dc,
                up: stat?.up, down: stat?.down, actualTotal: stat?.total,
                actualExpiryTime: stat?.expiryTime, enableClientStat: stat?.enable,
                inboundId: stat?.inboundId, clientTrafficId: stat?.id,
                originalIndex: index
            };
          });
          currentInbound.clientStats?.forEach(stat => {
            if (!mergedClients.find(mc => mc.email === stat.email)) {
              mergedClients.push({
                email: stat.email, up: stat.up, down: stat.down, actualTotal: stat.total,
                actualExpiryTime: stat.expiryTime, enableClientStat: stat.enable,
                inboundId: stat.inboundId, clientTrafficId: stat.id,
              });
            }
          });
          setDisplayClients(mergedClients);
        } else { setPageError('Inbound not found.'); setInbound(null); setDisplayClients([]); }
      } else { setPageError(response.message || 'Failed to fetch inbound data.'); setInbound(null); setDisplayClients([]); }
    } catch (err) { setPageError(err instanceof Error ? err.message : 'An unknown error occurred.'); setInbound(null); setDisplayClients([]); }
    finally { setIsLoading(false); }
  }, [isAuthenticated, inboundId]);

  useEffect(() => {
    if (!authLoading && isAuthenticated) fetchInboundAndClients();
    else if (!authLoading && !isAuthenticated) { setIsLoading(false); router.push('/auth/login'); }
  }, [isAuthenticated, authLoading, fetchInboundAndClients, router]);

  const openAddModal = () => {
    setEditingClient(null); setClientFormModalError(null); setIsClientFormModalOpen(true);
  };
  const openEditModal = (client: DisplayClient) => {
    setEditingClient(client); setClientFormModalError(null); setIsClientFormModalOpen(true);
  };
  const openShareModal = (client: ClientSetting) => { // ClientSetting is enough for link generation
    setSharingClient(client); setIsShareModalOpen(true);
  };


  const handleClientFormSubmit = async (submittedClientData: ClientSetting) => {
    if (!inbound) { setClientFormModalError("Inbound data not available."); return; }
    setClientFormModalLoading(true); setClientFormModalError(null); setActionError(null);
    try {
      let currentSettings: { clients?: ClientSetting[], [key:string]: unknown } = {};
      try { currentSettings = JSON.parse(inbound.settings || '{}'); }
      catch (e) { console.error("Corrupted inbound settings:", e); currentSettings.clients = []; }

      const updatedClients = [...(currentSettings.clients || [])];
      let clientIdentifierForApi: string | undefined;

      if (editingClient && editingClient.originalIndex !== undefined) {
        updatedClients[editingClient.originalIndex] = submittedClientData;
        clientIdentifierForApi = inbound.protocol === 'trojan' ? editingClient.password : editingClient.id;
         if (!clientIdentifierForApi && submittedClientData.password && inbound.protocol === 'trojan') clientIdentifierForApi = submittedClientData.password;
         if (!clientIdentifierForApi && submittedClientData.id && (inbound.protocol === 'vmess' || inbound.protocol === 'vless')) clientIdentifierForApi = submittedClientData.id;
      } else {
        if (updatedClients.some(c => c.email === submittedClientData.email)) {
            setClientFormModalError(`Client with email "${submittedClientData.email}" already exists.`);
            setClientFormModalLoading(false); return;
        }
        updatedClients.push(submittedClientData);
      }

      const updatedSettingsJson = JSON.stringify({ ...currentSettings, clients: updatedClients }, null, 2);
      const payloadForApi: Partial<Inbound> = { ...inbound, id: inbound.id, settings: updatedSettingsJson };

      let response;
      if (editingClient) {
        if (!clientIdentifierForApi) {
            clientIdentifierForApi = inbound.protocol === 'trojan' ? editingClient?.password : editingClient?.id;
        }
        if (!clientIdentifierForApi) {
            setClientFormModalError("Original client identifier for API is missing for editing.");
            setClientFormModalLoading(false); return;
        }
        response = await post<Inbound>(`/inbound/updateClient/${clientIdentifierForApi}`, payloadForApi);
      } else {
        response = await post<Inbound>('/inbound/addClient', payloadForApi);
      }

      if (response.success) {
        setIsClientFormModalOpen(false); setEditingClient(null);
        await fetchInboundAndClients();
      } else { setClientFormModalError(response.message || `Failed to ${editingClient ? 'update' : 'add'} client.`); }
    } catch (err) { setClientFormModalError(err instanceof Error ? err.message : `An error occurred.`); }
    finally { setClientFormModalLoading(false); }
  };

  const handleDeleteClient = async (clientToDelete: DisplayClient) => {
    if (!inbound || !clientToDelete.email) { setActionError("Client or Inbound data is missing."); return; }
    const clientApiId = clientToDelete.email;
    if (!window.confirm(`Delete client: ${clientToDelete.email}?`)) return;
    setCurrentAction(ClientAction.DELETING); setActionTargetEmail(clientToDelete.email); setActionError(null);
    try {
      const response = await post(`/inbound/${inbound.id}/delClient/${clientApiId}`, {});
      if (response.success) { await fetchInboundAndClients(); }
      else { setActionError(response.message || "Failed to delete client."); }
    } catch (err) { setActionError(err instanceof Error ? err.message : "Error deleting client."); }
    finally { setCurrentAction(ClientAction.NONE); setActionTargetEmail(null); }
  };

  const handleResetClientTraffic = async (clientToReset: DisplayClient) => {
    if (!inbound || !clientToReset.email) { setActionError("Client/Inbound data missing."); return; }
    if (!window.confirm(`Reset traffic for: ${clientToReset.email}?`)) return;
    setCurrentAction(ClientAction.RESETTING_TRAFFIC); setActionTargetEmail(clientToReset.email); setActionError(null);
    try {
      const response = await post(`/inbound/${inbound.id}/resetClientTraffic/${clientToReset.email}`, {});
      if (response.success) { await fetchInboundAndClients(); }
      else { setActionError(response.message || "Failed to reset traffic."); }
    } catch (err) { setActionError(err instanceof Error ? err.message : "Error resetting traffic."); }
    finally { setCurrentAction(ClientAction.NONE); setActionTargetEmail(null); }
  };

  const getClientIdentifier = (client: DisplayClient, proto: Protocol | undefined): string => proto === 'trojan' ? client.password || 'N/A' : client.id || 'N/A';
  const getClientIdentifierLabel = (proto: Protocol | undefined): string => proto === 'trojan' ? 'Password' : 'UUID';

  if (isLoading || authLoading) return <div className="p-4 text-center text-gray-700 dark:text-gray-300">Loading...</div>;
  if (pageError && !inbound) return <div className="p-4 text-red-500 dark:text-red-400 text-center">Error: {pageError}</div>;
  if (!inbound && !isLoading) return <div className="p-4 text-center text-gray-700 dark:text-gray-300">Inbound not found.</div>;

  const canManageClients = inbound && (inbound.protocol === 'vmess' || inbound.protocol === 'vless' || inbound.protocol === 'trojan' || inbound.protocol === 'shadowsocks');

  return (
    <div className="text-gray-800 dark:text-gray-200 p-2 md:p-0">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl md:text-3xl font-semibold">
          Clients for: <span className="text-primary-500 dark:text-primary-400">{inbound?.remark || `#${inbound?.id}`}</span>
           <span className="text-base ml-2 text-gray-500 dark:text-gray-400">({inbound?.protocol})</span>
        </h1>
        {canManageClients && (inbound?.protocol !== 'shadowsocks') &&
            (<button onClick={openAddModal} className={btnPrimaryStyles}>Add Client</button>)
        }
      </div>

      {pageError && inbound && <div className="mb-4 p-3 bg-yellow-100 text-yellow-700 dark:bg-yellow-800 dark:text-yellow-200 rounded-md">Page load error: {pageError} (stale data)</div>}
      {actionError && <div className="mb-4 p-3 bg-red-100 text-red-700 dark:bg-red-800 dark:text-red-200 rounded-md">Action Error: {actionError}</div>}

      {displayClients.length === 0 && !pageError && inbound?.protocol !== 'shadowsocks' && <p>No clients configured for this inbound.</p>}
      {inbound?.protocol === 'shadowsocks' &&
        <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            For Shadowsocks, client configuration (method and password) is part of the main inbound settings.
            The QR code / subscription link below uses these global settings.
        </p>
      }

      {(displayClients.length > 0 || inbound?.protocol === 'shadowsocks') && (
        <div className="overflow-x-auto bg-white dark:bg-gray-800 shadow-lg rounded-lg">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Email / Identifier</th>
                {(inbound?.protocol !== 'shadowsocks') && <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">{getClientIdentifierLabel(inbound?.protocol)}</th>}
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Traffic (Up/Down)</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Quota</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Expiry</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              { (inbound?.protocol === 'shadowsocks') ? (
                <tr className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">{inbound.remark || 'Shadowsocks Settings'}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{formatBytes(inbound.up || 0)} / {formatBytes(inbound.down || 0)}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{inbound.total > 0 ? formatBytes(inbound.total) : 'Unlimited'}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{inbound.expiryTime > 0 ? new Date(inbound.expiryTime).toLocaleDateString() : 'Never'}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm">
                    {inbound.enable ? <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800 dark:bg-green-700 dark:text-green-100">Enabled</span> : <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-red-100 text-red-800 dark:bg-red-700 dark:text-red-100">Disabled</span> }
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium space-x-2">
                     <button onClick={() => openShareModal({email: inbound.remark || 'ss_client'})} className={btnTextIndigoStyles} disabled={currentAction !== ClientAction.NONE}>QR / Link</button>
                     {/* No Edit/Delete/Reset for SS "client" as it's part of inbound config */}
                  </td>
                </tr>
              ) : displayClients.map((client) => {
                const clientActionTargetId = client.email;
                const isCurrentActionTarget = actionTargetEmail === clientActionTargetId;
                return (
                <tr key={client.email || client.id || client.password} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">{client.email}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-mono text-xs break-all">{getClientIdentifier(client, inbound?.protocol)}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{formatBytes(client.up || 0)} / {formatBytes(client.down || 0)}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                    { (client.totalGB !== undefined && client.totalGB > 0) ? formatBytes(client.totalGB * 1024 * 1024 * 1024) : (client.actualTotal !== undefined && client.actualTotal > 0) ? formatBytes(client.actualTotal) : 'Unlimited' }
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                    {client.actualExpiryTime && client.actualExpiryTime > 0 ? new Date(client.actualExpiryTime).toLocaleDateString() : client.expiryTime && client.expiryTime > 0 ? new Date(client.expiryTime).toLocaleDateString() + " (Def)" : 'Never'}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm">
                    {client.enableClientStat === undefined ? 'N/A' : client.enableClientStat ?
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800 dark:bg-green-700 dark:text-green-100">Enabled</span> :
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-red-100 text-red-800 dark:bg-red-700 dark:text-red-100">Disabled</span>
                    }
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium space-x-2">
                    <button onClick={() => openEditModal(client)} disabled={currentAction !== ClientAction.NONE} className={btnTextPrimaryStyles}>Edit</button>
                    <button onClick={() => handleDeleteClient(client)} disabled={currentAction !== ClientAction.NONE} className={btnTextDangerStyles}>
                        {isCurrentActionTarget && currentAction === ClientAction.DELETING ? 'Deleting...' : 'Delete'}
                    </button>
                    <button onClick={() => handleResetClientTraffic(client)} disabled={currentAction !== ClientAction.NONE} className={btnTextWarningStyles}>
                        {isCurrentActionTarget && currentAction === ClientAction.RESETTING_TRAFFIC ? 'Resetting...' : 'Reset Traffic'}
                    </button>
                    <button onClick={() => openShareModal(client)} className={btnTextIndigoStyles} disabled={currentAction !== ClientAction.NONE}>QR / Link</button>
                  </td>
                </tr>
              )})}
            </tbody>
          </table>
        </div>
      )}
      {isClientFormModalOpen && inbound && (
        <ClientFormModal isOpen={isClientFormModalOpen} onClose={() => { setIsClientFormModalOpen(false); setEditingClient(null); }}
          onSubmit={handleClientFormSubmit} protocol={inbound.protocol as Protocol}
          existingClient={editingClient} formError={clientFormModalError} isLoading={clientFormModalLoading}
        />
      )}
      {isShareModalOpen && inbound && (
        <ClientShareModal isOpen={isShareModalOpen} onClose={() => setIsShareModalOpen(false)}
          inbound={inbound} client={sharingClient}
        />
      )}
    </div>
  );
};
export default ManageClientsPage;
