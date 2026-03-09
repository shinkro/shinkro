import {SettingsSectionHeader, TMDBIcon, TVDBIcon} from "@screens/settings/components.tsx";
import {useMutation, useSuspenseQuery, useQueryClient} from "@tanstack/react-query";
import {MappingQueryOptions} from "@api/queries.ts";
import {APIClient} from "@api/APIClient.ts";
import {
    Button,
    Divider,
    Group,
    Stack,
    Switch,
    Text,
} from "@mantine/core";
import {MappingKeys} from "@api/query_keys.ts";
import {useDisclosure} from "@mantine/hooks";
import {useState} from "react";
import {MapDirSelect} from "@forms/settings/MapDirSelect.tsx";
import {displayNotification} from "@components/notifications";
import {ValidateMap} from "@app/types/Mapping";

export const MapSettings = () => {
    const {data: mapping} = useSuspenseQuery(MappingQueryOptions());
    const queryClient = useQueryClient();
    const [modalOpen, {open, close}] = useDisclosure(false);
    const [settingTarget, setSettingTarget] = useState<"tvdb" | "tmdb" | null>(null);

    const mutation = useMutation({
        mutationFn: APIClient.mapping.update,
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: MappingKeys.lists()});
        },
    });

    const validMutation = useMutation({
        mutationFn: (map: ValidateMap) => APIClient.mapping.validate(map),
        onSuccess: (_, variables) => {
            if (!mapping) return;

            const updated = {...mapping};

            if (variables.isTVDB) {
                updated.tvdb_path = variables.yamlPath;
                updated.tvdb_enabled = true;
            } else {
                updated.tmdb_path = variables.yamlPath;
                updated.tmdb_enabled = true;
            }

            mutation.mutate(updated);
            close();
        },
        onError: () => {
            displayNotification({
                type: "error",
                title: "Custom Map Validation Failed",
                message: "Modify your cutom map and retry.",
            });
        },
    });

    const handlePathSelect = (path: string) => {
        if (!mapping || !settingTarget) return;

        validMutation.mutate({
            isTVDB: settingTarget === "tvdb",
            yamlPath: path,
        });
    };

    return (
        <main>
            <SettingsSectionHeader
                title="Mapping"
                description="Set up custom mapping for anime matching here."
                link={"https://github.com/shinkro/community-mapping"}
                note={
                    <Stack>
                        <Text fw={600} size={"sm"}>
                            Community map(s) is disabled upon enabling custom map(s).
                        </Text>
                        <Text fw={600} size={"sm"}>
                            Instead of maintaining custom maps, you can consider contributing to the community maps by
                            clicking on this button.
                        </Text>
                    </Stack>
                }
            />
            <Stack gap="md" mt="md">
                    <Group justify="flex-start" align="center" wrap="nowrap">
                        <Text w={100} fw={600}>Enabled</Text>
                        <Text w={80} fw={600}>Type</Text>
                        <Text w={575} miw={80} fw={600} style={{flex: 1}}>Path</Text>
                        <Text w={200} fw={600}>Actions</Text>
                    </Group>

                    <Divider/>

                    {(["tvdb", "tmdb"] as const).map((type) => {
                        const isTVDB = type === "tvdb";
                        const enabled = isTVDB ? mapping.tvdb_enabled : mapping.tmdb_enabled;
                        const path = isTVDB ? mapping.tvdb_path : mapping.tmdb_path;
                        return (
                            <div key={type}>
                                <Group justify="flex-start" align="center" wrap="nowrap">
                                    <Switch
                                        size="sm"
                                        checked={enabled}
                                        disabled={!path}
                                        onChange={() => {
                                            const updated = {...mapping};
                                            if (isTVDB) updated.tvdb_enabled = !enabled;
                                            else updated.tmdb_enabled = !enabled;
                                            mutation.mutate(updated);
                                        }}
                                        w={82}
                                        ml={18}
                                    />
                                    {type === "tvdb" ? (
                                        <TVDBIcon/>
                                    ) : (
                                        <TMDBIcon/>
                                    )}
                                    <Text
                                        ml={40}
                                        miw={80}
                                        style={{
                                            flex: 1,
                                        }}
                                        truncate
                                    >
                                        <code>{path || "Not set"}</code>
                                    </Text>
                                    <Group gap="xs" w={284}>
                                        <Button
                                            size="xs"
                                            onClick={() => {
                                                setSettingTarget(type);
                                                open();
                                            }}
                                            w={100}
                                        >
                                            Set Path
                                        </Button>
                                        <Button
                                            size="xs"
                                            variant="default"
                                            color="gray"
                                            onClick={() => {
                                                const updated = {...mapping};
                                                if (isTVDB) {
                                                    updated.tvdb_path = "";
                                                    updated.tvdb_enabled = false;
                                                } else {
                                                    updated.tmdb_path = "";
                                                    updated.tmdb_enabled = false;
                                                }
                                                mutation.mutate(updated);
                                            }}
                                            w={100}
                                        >
                                            Reset
                                        </Button>
                                    </Group>
                                </Group>
                                <Divider mt={"md"}/>
                            </div>
                        );
                    })}
                </Stack>
            <MapDirSelect
                opened={modalOpen}
                onClose={close}
                onSelect={handlePathSelect}
            />
        </main>
    );
};