# e2e case for ippool

| case id | category  | description                                             | priority | status | other |
|---------|-----------|---------------------------------------------------------|----------|--------|-------|
| F00001  | ippool | assign ip to a pod for case ipv4, ipv6, dual-stack ip   | p2       | done   |       |
| F00002  | ippool | assign ip to 100 pod for case ipv4, ipv6, dual-stack ip   | p2       |       |       |
| F00003  | api | post /ipam/ip   | p2       |       |       |
| F00004  | api | get /ipam/ip   | p2       |       |       |
| F00005  | api | patch /ipam/ip   | p2       |       |       |
| F00006  | api | delete /ipam/ip   | p2       |       |       |
| F00007  | metric | check ip number    | p2       |       |       |
| F00008  | metric | check ip pool number   | p2       |       |       |
| F00009  | log | check spider-agent log   | p2       |       |       |
| F000010  | log | check spider-server log   | p2       |       |       |
| F000011  | log | check plugin log   | p2       |       |       |


case id	category	description	priority	status	other
F00001	ippool	assign ip to a pod for case ipv4, ipv6, dual-stack ip	p2	done	
F00002	ippool	assign ip to 100 pod for case ipv4, ipv6, dual-stack ip	p2		
F00003	ippool	单pod释放ip			
F00004	ippool	100pod释放			
F00005	api	post /ipam/ip	p2		
F00006	api	get /ipam/ip	p2		
F00007	api	patch /ipam/ip	p2		
F00008	api	delete /ipam/ip	p2		
F00009	metric	check ip number	p2		
F00010	metric	check ip pool number	p2		
F00011	log	check spider-agent log	p2		
F00012	log	check spider-server log	p2		
F00013	log	check plugin log	p2		![image](https://user-images.githubusercontent.com/31728060/166895559-604ef48b-28e3-4637-876a-afebe1d88ff2.png)

