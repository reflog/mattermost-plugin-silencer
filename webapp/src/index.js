import { id as pluginId } from './manifest';
import { executeCommand } from 'mattermost-redux/actions/integrations';
import { getUser } from 'mattermost-redux/selectors/entities/users';
import { postRemoved } from 'mattermost-redux/actions/posts';

function delay(ms) {
    return new Promise((resolve) => {
        setTimeout(() => {
            resolve();
        }, ms);
    })
}

export default class Plugin {
    silencedUsers = [];

    isSilenced = (state, user_id, my_user_id) => {
        if (user_id === my_user_id) return false;
        const user = getUser(state, user_id);
        if (!user) return false;
        return this.silencedUsers.indexOf(user.username) !== -1;
    }

    async initialize(registry, store) {
        registry.registerWebSocketEventHandler(
            'custom_' + pluginId + '_silencer_list_changed',
            (message) => {
                delay(1000).then(() => {
                    this.silencedUsers = message.data.list || [];
                    const posts = Object.values(store.getState().entities.posts.posts);
                    const currentChannelId = store.getState().entities.channels.currentChannelId;
                    const currentUserId = store.getState().entities.users.currentUserId;
                    for (const p of posts) {
                        const pu = getUser(store.getState(), p.user_id);
                        if (p.channel_id === currentChannelId && this.isSilenced(store.getState(), p.user_id, currentUserId)) {
                            store.dispatch(postRemoved(p));
                        }
                    }
                });
            },
        );
        registry.registerMessageWillFormatHook((data) => {
            if (this.isSilenced(store.getState(), data.user_id, store.getState().entities.users.currentUserId)) {
                store.dispatch(postRemoved(data));
                return "silenced"
            }
            return data.message;
        })

        registry.registerRootComponent(
            () => {
                delay(1000).then(() => store.dispatch(executeCommand("/silencer", {
                    channel_id: store.getState().entities.channels.currentChannelId,
                    team_id: store.getState().entities.teams.currentTeamId,
                })));
                return null;
            });

    }
}

window.registerPlugin(pluginId, new Plugin());
