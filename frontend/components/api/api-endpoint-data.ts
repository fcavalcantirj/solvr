import { EndpointGroup } from "./api-endpoint-types";
import { coreEndpointGroups } from "./api-endpoint-data-core";
import { contentEndpointGroups } from "./api-endpoint-data-content";
import { userEndpointGroups } from "./api-endpoint-data-user";
import { ipfsEndpointGroups } from "./api-endpoint-data-ipfs";

export const endpointGroups: EndpointGroup[] = [
  ...coreEndpointGroups,
  ...contentEndpointGroups,
  ...userEndpointGroups,
  ...ipfsEndpointGroups,
];
